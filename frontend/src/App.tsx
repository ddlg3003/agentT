import { Outlet } from "react-router-dom";

import { Sidebar } from "./features/digest/Sidebar";

/** App shell: persistent sidebar + routed main content. */
export function App() {
  return (
    <div className="flex h-screen overflow-hidden bg-slate-100 text-slate-900">
      <Sidebar />
      <main className="min-w-0 flex-1 overflow-hidden p-6">
        <Outlet />
      </main>
    </div>
  );
}
