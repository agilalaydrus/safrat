"use client";

import { StorefrontInquiry } from "@hajj-saas/proto-gen/hajj/v1/inquiry_pb";
import { IconInbox, IconMailForward, IconX } from "@tabler/icons-react";
import { useCallback, useEffect, useState } from "react";
import { inquiryClient } from "@/lib/rpc";
import { Button } from "@/components/ui/Button";

const formatDateTime = (date?: Date) =>
  date ? new Intl.DateTimeFormat("id-ID", { dateStyle: "medium", timeStyle: "short" }).format(date) : "—";
const nextKey = () => crypto.randomUUID();
const errorMessage = (error: unknown, fallback: string) => (error instanceof Error && error.message ? error.message : fallback);

// K2.5 (TUGAS-CORONG.md): visitor messages from the storefront's "Hubungi
// Kami" form, waiting to be turned into a crm_lead. "Jadikan Lead" is the
// one click that fills Source=Website/Campaign from the visitor's own utm
// tags — the whole point being staff no longer has to remember or guess it.
export default function InquiryInbox({ onConverted }: { onConverted: () => void }) {
  const [inquiries, setInquiries] = useState<StorefrontInquiry[]>([]);
  const [busyId, setBusyId] = useState("");
  const [notice, setNotice] = useState("");
  const [open, setOpen] = useState(true);

  const refresh = useCallback(async () => {
    try {
      const response = await inquiryClient.listInquiries({ status: "NEW" });
      setInquiries(response.inquiries);
    } catch {
      // Silent — this is a convenience panel, never blocks the CRM page itself.
    }
  }, []);

  useEffect(() => { void refresh(); }, [refresh]);

  async function convert(inquiry: StorefrontInquiry) {
    setBusyId(inquiry.id);
    try {
      await inquiryClient.convertInquiryToLead({ inquiryId: inquiry.id, idempotencyKey: nextKey() });
      setInquiries((value) => value.filter((item) => item.id !== inquiry.id));
      setNotice(`${inquiry.fullName} ditambahkan ke pipeline CRM.`);
      onConverted();
    } catch (error) {
      setNotice(`Gagal menjadikan lead. ${errorMessage(error, "Coba lagi.")}`);
    } finally {
      setBusyId("");
    }
  }

  async function dismiss(inquiry: StorefrontInquiry) {
    setBusyId(inquiry.id);
    try {
      await inquiryClient.dismissInquiry({ inquiryId: inquiry.id });
      setInquiries((value) => value.filter((item) => item.id !== inquiry.id));
    } catch (error) {
      setNotice(`Gagal mengabaikan pesan. ${errorMessage(error, "Coba lagi.")}`);
    } finally {
      setBusyId("");
    }
  }

  if (inquiries.length === 0) return null;

  return (
    <section className="tw-card crm-inbox" aria-labelledby="crm-inbox-title">
      <header className="crm-inbox__header">
        <div>
          <span className="crm-inbox__icon"><IconInbox size={19} /></span>
          <div>
            <h2 id="crm-inbox-title">{inquiries.length} pesan masuk dari storefront</h2>
            <p>Belum masuk pipeline. Jadikan lead untuk mulai ditindaklanjuti, atau abaikan bila bukan prospek.</p>
          </div>
        </div>
        <button type="button" onClick={() => setOpen((value) => !value)}>{open ? "Sembunyikan" : "Tampilkan"}</button>
      </header>
      {notice && <p className="crm-inbox__notice" role="status">{notice}</p>}
      {open && <div className="crm-inbox__list">
        {inquiries.map((inquiry) => (
          <article key={inquiry.id} className="crm-inbox__item">
            <div>
              <strong>{inquiry.fullName}</strong>
              <span>{[inquiry.phone, inquiry.email].filter(Boolean).join(" · ") || "Tanpa kontak"}</span>
              {inquiry.message && <p>{inquiry.message}</p>}
              <small>
                {formatDateTime(inquiry.createdAt?.toDate())}
                {inquiry.utmSource && ` · dari ${inquiry.utmSource}`}
                {inquiry.utmCampaign && ` · kampanye ${inquiry.utmCampaign}`}
              </small>
            </div>
            <div className="crm-inbox__actions">
              <Button variant="emerald" disabled={busyId === inquiry.id} onClick={() => void convert(inquiry)}>
                <IconMailForward size={15} />Jadikan lead
              </Button>
              <Button variant="ghost" disabled={busyId === inquiry.id} onClick={() => void dismiss(inquiry)}>
                <IconX size={15} />Abaikan
              </Button>
            </div>
          </article>
        ))}
      </div>}
    </section>
  );
}
