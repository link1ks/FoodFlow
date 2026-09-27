import { LocalPrices } from "../LocalPrices";
import { ShoppingBoard } from "../ShoppingBoard";
import { TodayBoard } from "../TodayBoard";
import { useBase, useData, type Household } from "../app/shared";
export function Today() {
  const { token, p } = useBase();
  return (
    <TodayBoard token={token} household={p.slice("/households/".length)} />
  );
}

export function Shopping() {
  const { token, p } = useBase();
  const household = useData<Household>("household", p);
  return (
    <ShoppingBoard
      token={token}
      household={p.slice("/households/".length)}
      canEdit={household.data?.role !== "viewer" && !!household.data}
    />
  );
}

export function Prices() {
  const { token, p } = useBase();
  return (
    <LocalPrices
      key={p}
      token={token}
      household={p.slice("/households/".length)}
    />
  );
}
