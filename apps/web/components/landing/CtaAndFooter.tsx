import Link from "next/link";
import { ArrowRight, Compass, Mail, MapPin, MessageCircle, Phone } from "lucide-react";
import { FOOTER_LAYANAN, FOOTER_MODULES, WA_NUMBER_DISPLAY, WA_SALES_LINK } from "./content";

export default function CtaAndFooter() {
  return (
    <>
      {/* CTA banner */}
      <section className="relative overflow-hidden bg-slate-900 py-16 text-white dark:bg-black">
        <div className="relative z-10 mx-auto max-w-5xl px-4 text-center sm:px-6 lg:px-8">
          <h2 className="text-3xl font-extrabold tracking-tight text-white sm:text-4xl">
            Siap Mengelola Musim Umrah 1447H dengan Lebih Tenang?
          </h2>
          <p className="mx-auto mt-4 max-w-2xl text-sm text-slate-300 sm:text-base">
            Bergabunglah dengan ratusan pimpinan travel yang telah memodernisasi operasional mereka. Mulai gratis hari ini.
          </p>
          <div className="mt-8 flex flex-col items-center justify-center gap-4 sm:flex-row">
            <Link
              href="/sign-up"
              className="flex w-full items-center justify-center gap-2 rounded-xl bg-emerald-400 px-8 py-4 text-sm font-bold text-slate-950 shadow-lg transition-all hover:bg-emerald-300 sm:w-auto"
            >
              <span>Coba Gratis</span>
              <ArrowRight className="h-4 w-4" />
            </Link>
            <a
              href={WA_SALES_LINK}
              target="_blank"
              rel="noreferrer"
              className="flex w-full items-center justify-center gap-2 rounded-xl border border-slate-700 bg-slate-800 px-7 py-4 text-sm font-bold text-white transition-all hover:bg-slate-700 sm:w-auto"
            >
              <MessageCircle className="h-4 w-4 text-emerald-400" />
              <span>Hubungi Sales</span>
            </a>
          </div>
        </div>
      </section>

      {/* Footer */}
      <footer className="border-t border-slate-800 bg-slate-950 py-12 text-xs text-slate-400">
        <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
          <div className="mb-8 grid grid-cols-1 gap-8 md:grid-cols-4">
            <div className="space-y-3">
              <div className="flex items-center gap-2 text-lg font-bold text-white">
                <Compass className="h-5 w-5 text-emerald-400" />
                <span>
                  Tawafiq<span className="text-emerald-400">Hub</span>
                </span>
              </div>
              <p className="leading-relaxed text-slate-400">
                Platform SaaS Sistem Operasi Terpadu Manajemen PPIU &amp; PIHK Resmi di Indonesia.
              </p>
            </div>

            <div>
              <h5 className="mb-3 font-bold uppercase tracking-wider text-white">Modul Utama</h5>
              <ul className="space-y-2">
                {FOOTER_MODULES.map(([label, href]) => (
                  <li key={label}>
                    <a href={href} className="hover:text-emerald-400">
                      {label}
                    </a>
                  </li>
                ))}
              </ul>
            </div>

            <div>
              <h5 className="mb-3 font-bold uppercase tracking-wider text-white">Layanan &amp; Regulasi</h5>
              <ul className="space-y-2">
                {FOOTER_LAYANAN.map(([label, href]) => (
                  <li key={label}>
                    <a href={href} className="hover:text-emerald-400">
                      {label}
                    </a>
                  </li>
                ))}
              </ul>
            </div>

            <div>
              <h5 className="mb-3 font-bold uppercase tracking-wider text-white">Kontak Layanan</h5>
              <ul className="space-y-2 text-slate-400">
                <li className="flex items-center gap-2">
                  <Mail className="h-4 w-4 text-emerald-400" /> halo@tawafiqhub.id
                </li>
                <li>
                  <a href={WA_SALES_LINK} target="_blank" rel="noreferrer" className="flex items-center gap-2 hover:text-emerald-400">
                    <Phone className="h-4 w-4 text-emerald-400" /> {WA_NUMBER_DISPLAY}
                  </a>
                </li>
                <li className="flex items-center gap-2">
                  <MapPin className="h-4 w-4 text-amber-300" /> Kantor: DKI Jakarta dan Kota Bekasi
                </li>
              </ul>
            </div>
          </div>

          <div className="flex flex-col items-center justify-between gap-4 border-t border-slate-900 pt-8 text-slate-500 sm:flex-row">
            <p>© {new Date().getFullYear()} Tawafiq Hub. Hak Cipta Dilindungi.</p>
            <div className="flex gap-4">
              <span className="cursor-pointer hover:text-slate-400">Kebijakan Privasi</span>
              <span>•</span>
              <span className="cursor-pointer hover:text-slate-400">Syarat &amp; Ketentuan</span>
            </div>
          </div>
        </div>
      </footer>

      {/* Floating WhatsApp */}
      <a
        href={WA_SALES_LINK}
        target="_blank"
        rel="noreferrer"
        aria-label="Hubungi Sales via WhatsApp"
        className="fixed bottom-6 right-6 z-40 flex items-center justify-center rounded-full border-2 border-white bg-emerald-600 p-3.5 text-white shadow-2xl transition-all hover:scale-110 hover:bg-emerald-500"
      >
        <MessageCircle className="h-6 w-6" />
      </a>
    </>
  );
}
