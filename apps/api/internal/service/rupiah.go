package service

import "strconv"

// rupiah formats an amount the way Indonesian money is written — Rp4.500.000,
// with a dot every three digits.
//
// It exists because these strings are read by people: they end up in audit
// entries, in the reason on a held transaction, and in error messages shown
// straight to an operator. A bare %d produced "Rp4200000" next to a UI that
// everywhere else says "Rp4.500.000", which was spotted only once the screen
// was actually looked at.
//
// Negative amounts keep the sign outside the currency, as "-Rp1.000".
func rupiah(amount int64) string {
	sign := ""
	if amount < 0 {
		sign = "-"
		amount = -amount
	}
	digits := strconv.FormatInt(amount, 10)
	// Walk from the right so the leftmost group is the short one.
	var grouped []byte
	for index, count := len(digits)-1, 0; index >= 0; index, count = index-1, count+1 {
		if count > 0 && count%3 == 0 {
			grouped = append(grouped, '.')
		}
		grouped = append(grouped, digits[index])
	}
	for left, right := 0, len(grouped)-1; left < right; left, right = left+1, right-1 {
		grouped[left], grouped[right] = grouped[right], grouped[left]
	}
	return sign + "Rp" + string(grouped)
}
