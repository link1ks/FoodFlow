import React from "react";
import { createRoot } from "react-dom/client";
import { QueryClientProvider } from "@tanstack/react-query";
import { Auth } from "./Auth";
import { useUI } from "./store";
import { HouseholdGate } from "./app/AppShell";
import { NatureBackdrop } from "./NatureTheme";
import { qc } from "./app/shared";
import "./style.css";
import "./atmosphere.css";
function Root() {
  const token = useUI((s) => s.token);
  return (
    <div className="foodflow-root">
      <NatureBackdrop />
      {token ? <HouseholdGate /> : <Auth />}
    </div>
  );
}
createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <QueryClientProvider client={qc}>
      <Root />
    </QueryClientProvider>
  </React.StrictMode>,
);
