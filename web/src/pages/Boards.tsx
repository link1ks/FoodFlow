import { LocalPrices } from "../LocalPrices";
import { ShoppingBoard } from "../ShoppingBoard";
import { KitchenHome } from "../KitchenHome";
import { Button } from "../ui";
import { useUI } from "../store";
import { useBase, useData, type Household } from "../app/shared";
export function Today({ name }: { name: string }) {
  const { token, p } = useBase();
  return (
    <KitchenHome
      key={p}
      name={name}
      token={token}
      household={p.slice("/households/".length)}
    />
  );
}

export function Shopping() {
  const { token, p } = useBase();
  const household = useData<Household>("household", p);
  return (
    <>
      <Button
        className="mb-4"
        variant="outline"
        onClick={() => useUI.getState().setPage("prices")}
      >
        查菜价 · 城市参考价格
      </Button>
      <ShoppingBoard
        token={token}
        household={p.slice("/households/".length)}
        canEdit={household.data?.role !== "viewer" && !!household.data}
      />
    </>
  );
}

export function Prices() {
  const { token, p } = useBase();
  return (
    <>
      <Button
        className="mb-4"
        variant="outline"
        onClick={() => useUI.getState().setPage("shopping")}
      >
        返回采购清单
      </Button>
      <LocalPrices
        key={p}
        token={token}
        household={p.slice("/households/".length)}
      />
    </>
  );
}
