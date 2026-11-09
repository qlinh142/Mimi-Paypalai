/**
 * Bảng Lịch sử mua hàng bằng AG Grid (tool sponsor): sắp xếp theo cột, lọc nhanh,
 * xuất CSV. Tải riêng (lazy) để màn danh sách thường không phải tải AG Grid.
 */
import { useMemo, useRef } from "react";
import { useNavigate } from "react-router-dom";
import { AgGridReact } from "ag-grid-react";
import { AllCommunityModule, ModuleRegistry, themeQuartz, type ColDef } from "ag-grid-community";
import type { HistoryRow } from "../api/types";
import { m15 } from "../i18n/m15";
import { formatVnd } from "../lib/text";
import { historyLink } from "../screens/PurchaseHistory";

ModuleRegistry.registerModules([AllCommunityModule]);

const theme = themeQuartz.withParams({ fontFamily: "inherit", fontSize: 13, headerFontWeight: 600, spacing: 6 });

type Row = { id: string; to: string; day: string; title: string; kind: string; items: number; total: number; on: string };

const pad = (n: number) => String(n).padStart(2, "0");
const vnDay = (iso: string) => {
  const d = new Date(new Date(iso).getTime() + 7 * 3_600_000);
  return `${d.getUTCFullYear()}-${pad(d.getUTCMonth() + 1)}-${pad(d.getUTCDate())}`;
};
/** "2026-11-03" → "03/11/26": gọn cho màn hẹp, vẫn sắp xếp theo chuỗi ISO gốc. */
const shortDay = (iso: string) => `${iso.slice(8, 10)}/${iso.slice(5, 7)}/${iso.slice(2, 4)}`;

export default function HistoryGrid({ rows, titleOf }: { rows: HistoryRow[]; titleOf: (r: HistoryRow) => string }) {
  const nav = useNavigate();
  const grid = useRef<AgGridReact<Row>>(null);
  const g = m15.history.grid;
  const data = useMemo<Row[]>(
    () =>
      rows.map((r) => ({
        id: r.receipt_id,
        to: historyLink(r),
        day: vnDay(r.purchased_at),
        title: titleOf(r),
        kind: g.kinds[r.kind ?? "own"],
        items: r.item_names.length,
        total: r.total_vnd,
        on: r.purchased_at,
      })),
    [rows, titleOf, g],
  );
  const cols = useMemo<ColDef<Row>[]>(
    () => [
      { field: "day", headerName: g.day, sort: "desc", width: 92, valueFormatter: (p) => (p.value ? shortDay(p.value) : "") },
      { field: "title", headerName: g.where, flex: 1, minWidth: 110, filter: true },
      {
        field: "total", headerName: g.total, width: 118, type: "numericColumn", filter: "agNumberColumnFilter",
        valueFormatter: (p) => (p.data ? formatVnd(p.data.total, p.data.on) : ""),
      },
      { field: "kind", headerName: g.kind, width: 110, filter: true },
      { field: "items", headerName: g.items, width: 80, type: "numericColumn" },
    ],
    [g],
  );
  return (
    <div className="hgrid">
      <div className="hgrid__bar">
        <span className="hint">{g.hint}</span>
        <button type="button" className="btn btn--outline" onClick={() => grid.current?.api.exportDataAsCsv({ fileName: "mimi-lich-su-mua-hang.csv" })}>
          {g.csv}
        </button>
      </div>
      <div className="hgrid__table">
        <AgGridReact<Row>
          ref={grid}
          theme={theme}
          rowData={data}
          columnDefs={cols}
          getRowId={(p) => p.data.id}
          defaultColDef={{ sortable: true, resizable: true }}
          domLayout="autoHeight"
          pagination
          paginationPageSize={20}
          paginationPageSizeSelector={false}
          onRowClicked={(e) => e.data && nav(e.data.to)}
          rowStyle={{ cursor: "pointer" }}
          localeText={g.locale}
        />
      </div>
    </div>
  );
}
