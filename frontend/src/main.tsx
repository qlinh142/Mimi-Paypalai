import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import "./styles/tokens.css";
import "./styles/base.css";
import "./styles/screens.css";
import "./styles/m15.css";
import { App } from "./app/App";
import { startPwaUpdates } from "./app/pwaUpdate";

startPwaUpdates();

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
