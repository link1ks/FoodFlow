import { create } from "zustand";
type State = {
  token: string;
  household: string;
  page: string;
  setToken: (v: string) => void;
  setHousehold: (v: string) => void;
  setPage: (v: string) => void;
};
export const useUI = create<State>((set) => ({
  token: localStorage.getItem("ff_token") || "",
  // Ignore legacy family selection once; explicit choices survive refreshes.
  household: localStorage.getItem("ff_workspace_v2") || "",
  page: "today",
  setToken: (v) => {
    localStorage.setItem("ff_token", v);
    localStorage.removeItem("ff_workspace_v2");
    localStorage.removeItem("ff_household");
    set({ token: v, household: "", page: "today" });
  },
  setHousehold: (v) => {
    localStorage.setItem("ff_workspace_v2", v);
    localStorage.setItem("ff_household", v);
    set({ household: v });
  },
  setPage: (v) => set({ page: v }),
}));
