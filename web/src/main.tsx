import React from "react";
import { createRoot } from "react-dom/client";
import { QueryClientProvider } from "@tanstack/react-query";
import { Auth } from "./Auth";
import { useUI } from "./store";
import { HouseholdGate } from "./app/AppShell";
import { qc } from "./app/shared";
import "./style.css";
function Root() {
  const token = useUI((s) => s.token);
  return token ? <HouseholdGate /> : <Auth />;
}
createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <QueryClientProvider client={qc}>
      <Root />
    </QueryClientProvider>
  </React.StrictMode>,
);
