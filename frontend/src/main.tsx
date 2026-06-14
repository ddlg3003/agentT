import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";

import "./index.css";
import { App } from "./App";
import { DigestPage } from "./features/digest/DigestPage";
import { MonthlyPage } from "./features/monthly/MonthlyPage";

const queryClient = new QueryClient({
  defaultOptions: { queries: { refetchOnWindowFocus: false, retry: 1 } },
});

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Routes>
          <Route element={<App />}>
            <Route index element={<Navigate to="/digests" replace />} />
            <Route path="digests" element={<DigestPage />} />
            <Route path="digests/:date" element={<DigestPage />} />
            <Route path="report" element={<MonthlyPage />} />
          </Route>
          <Route path="*" element={<Navigate to="/digests" replace />} />
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
  </StrictMode>,
);
