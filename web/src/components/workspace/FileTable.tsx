import { useState } from "react";
import { Link } from "react-router-dom";
import type { WorkspaceItem } from "../../api/types";
import { withQuery } from "../../lib/url";
import { FileSize } from "../data/FileSize";
import { TypeBadge } from "../data/TypeBadge";
import { Button } from "../ui/Button";
import { CopyButton } from "../ui/CopyButton";
import { Modal } from "../ui/Modal";

type Viewer = {
  href: string;
};

function viewerFor(item: WorkspaceItem): Viewer | null {
  if (item.name.endsWith(".md")) {
    return { href: withQuery("/docs", { path: item.name }) };
  }
  if (item.type === "suite") {
    return { href: withQuery("/suite", { path: item.name }) };
  }
  if (item.type === "history") {
    return { href: withQuery("/history", { path: item.name }) };
  }
  return null;
}

export function FileTable({ items }: { items: WorkspaceItem[] }) {
  const [selected, setSelected] = useState<WorkspaceItem | null>(null);

  if (items.length === 0) {
    return <p className="st-workspace__empty">Không có tệp phù hợp.</p>;
  }

  return (
    <>
      <div className="st-file-table__scroll">
        <table className="st-file-table">
          <thead>
            <tr>
              <th scope="col">Tên tệp</th>
              <th scope="col">Loại</th>
              <th scope="col">Kích thước</th>
              <th scope="col">
                <span className="st-visually-hidden">Hành động</span>
              </th>
            </tr>
          </thead>
          <tbody>
            {items.map((item) => {
              const viewer = viewerFor(item);

              return (
                <tr key={item.path}>
                  <td>
                    <p className="st-file-table__name">{item.name}</p>
                    <p className="st-file-table__path">{item.path}</p>
                  </td>
                  <td>
                    <TypeBadge type={item.type} />
                  </td>
                  <td>
                    <FileSize bytes={item.size} />
                  </td>
                  <td className="st-file-table__action">
                    {viewer ? (
                      <Link
                        className="st-btn st-btn--secondary st-btn--sm"
                        to={viewer.href}
                        aria-label={`Mở ${item.name}`}
                      >
                        Mở
                      </Link>
                    ) : (
                      <Button
                        variant="secondary"
                        size="sm"
                        aria-label={`Mở ${item.name}`}
                        onClick={() => setSelected(item)}
                      >
                        Mở
                      </Button>
                    )}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>

      <Modal
        open={selected !== null}
        title="Tệp này chưa có trình xem"
        onClose={() => setSelected(null)}
        footer={
          <Button variant="secondary" onClick={() => setSelected(null)}>
            Đóng
          </Button>
        }
      >
        {selected && (
          <div className="st-file-table__modal">
            <p>Đường dẫn tệp:</p>
            <code>{selected.path}</code>
            <CopyButton value={selected.path} label={`đường dẫn ${selected.name}`} />
          </div>
        )}
      </Modal>
    </>
  );
}
