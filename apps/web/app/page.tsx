"use client";

import { useState } from "react";
import { ThemeProvider } from "@/components/landing/ThemeProvider";
import Navbar from "@/components/landing/Navbar";
import Hero from "@/components/landing/Hero";
import ProblemSolution from "@/components/landing/ProblemSolution";
import ModuleTabs from "@/components/landing/ModuleTabs";
import RoiCalculator from "@/components/landing/RoiCalculator";
import Pricing from "@/components/landing/Pricing";
import Testimonials from "@/components/landing/Testimonials";
import Faq from "@/components/landing/Faq";
import CtaAndFooter from "@/components/landing/CtaAndFooter";
import DemoModal from "@/components/landing/DemoModal";

export default function LandingPage() {
  const [demoOpen, setDemoOpen] = useState(false);
  const openDemo = () => setDemoOpen(true);

  return (
    <ThemeProvider>
      <div className="landing-scope min-h-screen bg-slate-50 text-slate-900 antialiased selection:bg-emerald-100 selection:text-emerald-900 dark:bg-slate-950 dark:text-white">
        <Navbar onOpenDemo={openDemo} />
        <Hero onOpenDemo={openDemo} />
        <ProblemSolution />
        <ModuleTabs />
        <RoiCalculator onOpenDemo={openDemo} />
        <Pricing onOpenDemo={openDemo} />
        <Testimonials />
        <Faq />
        <CtaAndFooter onOpenDemo={openDemo} />
        {demoOpen && <DemoModal onClose={() => setDemoOpen(false)} />}
      </div>
    </ThemeProvider>
  );
}
