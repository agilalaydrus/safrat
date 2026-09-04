package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"strconv"
	"time"

	"github.com/hajj-saas/api/internal/domain"
)

// buildOperatorExport is the portability right, assembled from the same
// repositories every screen reads from — never from raw SQL against the
// tables directly. Several columns (passport numbers, identity numbers) are
// sealed at rest and only readable through the repository layer that holds
// the decryption key; a hand-rolled query here would hand the operator back
// their own data as ciphertext.
//
// Scoped to pilgrims, their orders, the products and seasons behind them —
// the data a portability request is actually about. Agents, groups and
// kloters are organisational structure rather than personal data and are left
// for a later pass; noted honestly in the task file rather than silently
// dropped.
func (s *OrderService) BuildExport(ctx context.Context, operatorID string) ([]byte, error) {
	seasons, err := s.seasonRepository.ListByOperatorID(ctx, operatorID)
	if err != nil {
		return nil, fmt.Errorf("list seasons: %w", err)
	}

	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)

	if err := writeCSVEntry(writer, "seasons.csv",
		[]string{"id", "name", "type", "start_date", "end_date", "capacity", "is_active"},
		func(row func([]string)) error {
			for _, season := range seasons {
				row([]string{season.ID, season.Name, string(season.Type),
					formatTime(season.StartDate), formatTime(season.EndDate),
					strconv.Itoa(int(season.Capacity)), strconv.FormatBool(season.IsActive)})
			}
			return nil
		}); err != nil {
		return nil, err
	}

	var products []*domain.Product
	for _, season := range seasons {
		seasonProducts, err := s.productRepository.ListBySeasonID(ctx, operatorID, season.ID)
		if err != nil {
			return nil, fmt.Errorf("list products for season %s: %w", season.ID, err)
		}
		products = append(products, seasonProducts...)
	}
	if err := writeCSVEntry(writer, "products.csv",
		[]string{"id", "season_id", "name", "category", "type", "price_idr", "is_active"},
		func(row func([]string)) error {
			for _, product := range products {
				row([]string{product.ID, product.SeasonID, product.Name, product.Category, product.Type,
					strconv.FormatInt(product.PriceIDR, 10), strconv.FormatBool(product.IsActive)})
			}
			return nil
		}); err != nil {
		return nil, err
	}

	if err := writeCSVEntry(writer, "pilgrims.csv",
		[]string{"id", "season_id", "full_name", "passport_number", "nationality", "date_of_birth",
			"gender", "phone", "email", "payment_status", "status", "is_substituted", "created_at"},
		func(row func([]string)) error {
			for _, season := range seasons {
				const pageSize = int32(500)
				for offset := int32(0); ; offset += pageSize {
					page, err := s.pilgrimRepository.List(ctx, operatorID, season.ID, pageSize, offset)
					if err != nil {
						return fmt.Errorf("list pilgrims for season %s: %w", season.ID, err)
					}
					for _, pilgrim := range page {
						row([]string{pilgrim.ID, pilgrim.SeasonID, pilgrim.FullName, pilgrim.PassportNumber,
							pilgrim.Nationality, formatTime(pilgrim.DateOfBirth), pilgrim.Gender, pilgrim.Phone,
							pilgrim.Email, pilgrim.PaymentStatus, pilgrim.Status,
							strconv.FormatBool(pilgrim.IsSubstituted), formatTime(pilgrim.CreatedAt)})
					}
					if len(page) < int(pageSize) {
						break
					}
				}
			}
			return nil
		}); err != nil {
		return nil, err
	}

	if err := writeCSVEntry(writer, "orders.csv",
		[]string{"id", "season_id", "pilgrim_name", "product_name", "status", "total_price_idr",
			"paid_amount_idr", "room_tier", "paid_at", "created_at"},
		func(row func([]string)) error {
			for _, season := range seasons {
				const pageSize = int32(500)
				for offset := int32(0); ; offset += pageSize {
					page, err := s.orderRepository.ListBySeason(ctx, operatorID, season.ID, pageSize, offset)
					if err != nil {
						return fmt.Errorf("list orders for season %s: %w", season.ID, err)
					}
					for _, order := range page {
						paid := ""
						if order.PaidAmountIDR != nil {
							paid = strconv.FormatInt(*order.PaidAmountIDR, 10)
						}
						paidAt := ""
						if order.PaidAt != nil {
							paidAt = formatTime(*order.PaidAt)
						}
						row([]string{order.ID, order.SeasonID, order.PilgrimName, order.ProductName, order.Status,
							strconv.FormatInt(order.TotalPriceIDR, 10), paid, "", paidAt, formatTime(order.CreatedAt)})
					}
					if len(page) < int(pageSize) {
						break
					}
				}
			}
			return nil
		}); err != nil {
		return nil, err
	}

	readme := "Ekspor data " + time.Now().Format("2006-01-02") + "\r\n\r\n" +
		"Berkas ini memuat data yang tersimpan atas nama travel Anda di TawafiqHub: musim, paket, jamaah, dan " +
		"transaksi. Nomor identitas dan paspor ditampilkan dalam bentuk aslinya, bukan bentuk tersegel yang " +
		"tersimpan di database kami.\r\n\r\n" +
		"Belum termasuk dalam ekspor ini: agen, grup, dan kloter — informasi struktur organisasi, bukan data " +
		"pribadi jamaah. Kalau Anda membutuhkannya, hubungi dukungan.\r\n"
	if err := writeTextEntry(writer, "BACA-DULU.txt", readme); err != nil {
		return nil, err
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close zip: %w", err)
	}
	return buffer.Bytes(), nil
}

func writeCSVEntry(writer *zip.Writer, name string, header []string, fill func(row func([]string)) error) error {
	part, err := writer.Create(name)
	if err != nil {
		return fmt.Errorf("create %s: %w", name, err)
	}
	// The BOM is what keeps Excel on Windows from reading Indonesian names
	// with diacritics as Latin-1 — the same reason the manifest and roomlist
	// exports carry one.
	if _, err := part.Write([]byte("\xEF\xBB\xBF")); err != nil {
		return err
	}
	csvWriter := csv.NewWriter(part)
	if err := csvWriter.Write(header); err != nil {
		return fmt.Errorf("write header for %s: %w", name, err)
	}
	var writeErr error
	if err := fill(func(row []string) {
		if writeErr != nil {
			return
		}
		writeErr = csvWriter.Write(row)
	}); err != nil {
		return err
	}
	if writeErr != nil {
		return fmt.Errorf("write row for %s: %w", name, writeErr)
	}
	csvWriter.Flush()
	return csvWriter.Error()
}

func writeTextEntry(writer *zip.Writer, name, content string) error {
	part, err := writer.Create(name)
	if err != nil {
		return err
	}
	_, err = part.Write([]byte(content))
	return err
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}
