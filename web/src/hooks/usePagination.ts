import { useState, useMemo, useEffect } from 'react';

export function usePagination<T>(items: T[] = [], initialPageSize: number = 10) {
  const safeItems = Array.isArray(items) ? items : [];
  const [currentPage, setCurrentPage] = useState<number>(1);
  const [pageSize, setPageSize] = useState<number>(initialPageSize);

  const totalPages = Math.max(1, Math.ceil(safeItems.length / pageSize));

  // If items list shrinks (e.g. from filtering) so that currentPage is out of bounds, reset safely
  useEffect(() => {
    if (currentPage > totalPages) {
      setCurrentPage(totalPages);
    }
  }, [safeItems.length, totalPages, currentPage]);

  const validPage = Math.min(Math.max(1, currentPage), totalPages);

  const paginatedItems = useMemo(() => {
    const start = (validPage - 1) * pageSize;
    return safeItems.slice(start, start + pageSize);
  }, [safeItems, validPage, pageSize]);

  return {
    currentPage: validPage,
    pageSize,
    totalItems: safeItems.length,
    totalPages,
    paginatedItems,
    setCurrentPage,
    setPageSize,
  };
}
