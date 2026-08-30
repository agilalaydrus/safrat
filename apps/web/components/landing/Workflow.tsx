import { IconArrowDownRight, IconCircleCheck, IconCornerDownRight } from "@tabler/icons-react";
import { WORKFLOW } from "./content";

export default function Workflow() {
  return (
    <section id="cara-kerja" className="landing-section landing-workflow">
      <div className="landing-container landing-workflow-layout">
        <div className="landing-workflow-intro">
          <IconCircleCheck size={38} stroke={1.4} />
          <h2>Satu alur kerja dari pendaftaran sampai pulang.</h2>
          <p>Tim tidak perlu membangun ulang data yang sama di setiap fase perjalanan.</p>
          <a href="#harga">Lihat pilihan paket <IconArrowDownRight size={18} /></a>
        </div>

        <div className="landing-workflow-track">
          {WORKFLOW.map((item) => (
            <article key={item.title}>
              <span><IconCornerDownRight size={18} /></span>
              <div><h3>{item.title}</h3><p>{item.description}</p></div>
            </article>
          ))}
        </div>
      </div>
    </section>
  );
}
