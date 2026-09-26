import React from 'react';
import { ChevronLeft, ChevronRight, ChevronsLeft, ChevronsRight } from 'lucide-react';

export interface PaginationProps {
  currentPage: number;
  totalItems: number;
  pageSize: number;
  onPageChange: (page: number) => void;
  onPageSizeChange: (pageSize: number) => void;
  pageSizeOptions?: number[];
  className?: string;
}

export const Pagination: React.FC<PaginationProps> = ({
  currentPage,
  totalItems,
  pageSize,
  onPageChange,
  onPageSizeChange,
  pageSizeOptions = [10, 25, 50, 100],
  className = '',
}) => {
  const totalPages = Math.max(1, Math.ceil(totalItems / pageSize));
  const safeCurrentPage = Math.min(Math.max(1, currentPage), totalPages);

  const startItem = totalItems === 0 ? 0 : (safeCurrentPage - 1) * pageSize + 1;
  const endItem = Math.min(totalItems, safeCurrentPage * pageSize);

  // Generate page numbers with ellipsis
  const getPageNumbers = () => {
    const pages: (number | string)[] = [];
    if (totalPages <= 7) {
      for (let i = 1; i <= totalPages; i++) {
        pages.push(i);
      }
    } else {
      if (safeCurrentPage <= 4) {
        pages.push(1, 2, 3, 4, 5, '...', totalPages);
      } else if (safeCurrentPage >= totalPages - 3) {
        pages.push(1, '...', totalPages - 4, totalPages - 3, totalPages - 2, totalPages - 1, totalPages);
      } else {
        pages.push(1, '...', safeCurrentPage - 1, safeCurrentPage, safeCurrentPage + 1, '...', totalPages);
      }
    }
    return pages;
  };

  const handlePageChange = (p: number) => {
    if (p >= 1 && p <= totalPages && p !== safeCurrentPage) {
      onPageChange(p);
    }
  };

  return (
    <div
      className={`flex flex-col sm:flex-row items-center justify-between gap-4 px-4 py-3 bg-obsidian-950/80 border-t border-obsidian-800 text-xs font-mono text-slate-400 ${className}`}
    >
      {/* Left: Info and Page Size Selector */}
      <div className="flex flex-wrap items-center gap-4">
        <span>
          Showing <span className="font-bold text-slate-200">{startItem}</span> to{' '}
          <span className="font-bold text-slate-200">{endItem}</span> of{' '}
          <span className="font-bold text-slate-200">{totalItems}</span> entries
        </span>

        <div className="flex items-center space-x-1.5">
          <label htmlFor="pageSizeSelect" className="text-slate-500">
            Per page:
          </label>
          <select
            id="pageSizeSelect"
            value={pageSize}
            onChange={(e) => {
              const newSize = Number(e.target.value);
              onPageSizeChange(newSize);
              onPageChange(1); // Reset to first page
            }}
            className="px-2 py-1 rounded bg-obsidian-900 border border-obsidian-700 text-slate-200 font-bold focus:outline-none focus:border-emerald-500/60"
          >
            {pageSizeOptions.map((opt) => (
              <option key={opt} value={opt}>
                {opt}
              </option>
            ))}
          </select>
        </div>
      </div>

      {/* Right: Navigation Controls */}
      <div className="flex items-center space-x-1">
        {/* First Page */}
        <button
          onClick={() => handlePageChange(1)}
          disabled={safeCurrentPage === 1}
          title="First Page"
          className="p-1.5 rounded border border-obsidian-800 bg-obsidian-900 hover:bg-obsidian-800 text-slate-400 hover:text-slate-200 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
        >
          <ChevronsLeft className="w-4 h-4" />
        </button>

        {/* Previous Page */}
        <button
          onClick={() => handlePageChange(safeCurrentPage - 1)}
          disabled={safeCurrentPage === 1}
          title="Previous Page"
          className="p-1.5 rounded border border-obsidian-800 bg-obsidian-900 hover:bg-obsidian-800 text-slate-400 hover:text-slate-200 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
        >
          <ChevronLeft className="w-4 h-4" />
        </button>

        {/* Page Numbers */}
        <div className="flex items-center space-x-1 px-1">
          {getPageNumbers().map((item, idx) => {
            if (typeof item === 'string') {
              return (
                <span key={`ellipsis-${idx}`} className="px-1 text-slate-600 select-none">
                  …
                </span>
              );
            }
            const isActive = item === safeCurrentPage;
            return (
              <button
                key={`page-${item}`}
                onClick={() => handlePageChange(item)}
                className={`min-w-[28px] h-7 px-1.5 rounded text-xs font-bold transition-all border ${
                  isActive
                    ? 'bg-emerald-500/20 border-emerald-500/50 text-emerald-400 shadow-[0_0_10px_rgba(16,185,129,0.2)]'
                    : 'bg-obsidian-900 border-obsidian-800 hover:bg-obsidian-800 text-slate-400 hover:text-slate-200'
                }`}
              >
                {item}
              </button>
            );
          })}
        </div>

        {/* Next Page */}
        <button
          onClick={() => handlePageChange(safeCurrentPage + 1)}
          disabled={safeCurrentPage === totalPages || totalPages === 0}
          title="Next Page"
          className="p-1.5 rounded border border-obsidian-800 bg-obsidian-900 hover:bg-obsidian-800 text-slate-400 hover:text-slate-200 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
        >
          <ChevronRight className="w-4 h-4" />
        </button>

        {/* Last Page */}
        <button
          onClick={() => handlePageChange(totalPages)}
          disabled={safeCurrentPage === totalPages || totalPages === 0}
          title="Last Page"
          className="p-1.5 rounded border border-obsidian-800 bg-obsidian-900 hover:bg-obsidian-800 text-slate-400 hover:text-slate-200 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
        >
          <ChevronsRight className="w-4 h-4" />
        </button>
      </div>
    </div>
  );
};
