"use client";

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

export default function LandingPage() {
  return (
    <ThemeProvider>
      <div className="landing-scope min-h-screen bg-slate-50 text-slate-900 antialiased selection:bg-emerald-100 selection:text-emerald-900 dark:bg-slate-950 dark:text-white">
        <Navbar />
        <Hero />
        <ProblemSolution />
        <ModuleTabs />
        <RoiCalculator />
        <Pricing />
        <Testimonials />
        <Faq />
        <CtaAndFooter />
      </div>
    </ThemeProvider>
  );
}
