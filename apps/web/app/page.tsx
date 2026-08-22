"use client";

import { useState } from "react";
import { ThemeProvider } from "@/components/landing/ThemeProvider";
import Navbar from "@/components/landing/Navbar";
import Hero from "@/components/landing/Hero";
import DashboardPreview from "@/components/landing/DashboardPreview";
import ProblemSolution from "@/components/landing/ProblemSolution";
import FeatureMatrix from "@/components/landing/FeatureMatrix";
import Simulator from "@/components/landing/Simulator";
import RoiCalculator from "@/components/landing/RoiCalculator";
import Testimonials from "@/components/landing/Testimonials";
import Faq from "@/components/landing/Faq";
import CtaAndFooter from "@/components/landing/CtaAndFooter";
import DemoModal from "@/components/landing/DemoModal";

export default function LandingPage() {
  const [demoOpen, setDemoOpen] = useState(false);

  return (
    <ThemeProvider>
      <div className="landing-scope min-h-screen bg-slate-50 text-slate-900 dark:bg-slate-950 dark:text-white">
        <Navbar onOpenDemo={() => setDemoOpen(true)} />
        <Hero />
        <DashboardPreview />
        <ProblemSolution />
        <FeatureMatrix />
        <Simulator />
        <RoiCalculator />
        <Testimonials />
        <Faq />
        <CtaAndFooter onOpenDemo={() => setDemoOpen(true)} />
        {demoOpen && <DemoModal onClose={() => setDemoOpen(false)} />}
      </div>
    </ThemeProvider>
  );
}
