import { useCallback, useMemo } from 'react';
import { AgGridReact } from 'ag-grid-react';
import type { ColDef, IServerSideDatasource, IServerSideGetRowsParams } from 'ag-grid-community';
import 'ag-grid-community/styles/ag-grid.css';
import 'ag-grid-community/styles/ag-theme-alpine.css';

const API_URL = import.meta.env.VITE_API_URL ?? 'http://localhost:8080';

function formatCurrency(value: unknown): string {
  if (value == null) return '';
  return new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD' }).format(
    Number(value),
  );
}

function formatDate(value: unknown): string {
  if (value == null) return '';
  return new Date(String(value)).toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  });
}

export default function ProductGrid() {
  const columnDefs = useMemo<ColDef[]>(
    () => [
      {
        field: 'name',
        headerName: 'Name',
        minWidth: 220,
        filter: 'agTextColumnFilter',
      },
      {
        field: 'category',
        headerName: 'Category',
        filter: 'agTextColumnFilter',
        enableRowGroup: true,
        rowGroup: false,
      },
      {
        field: 'subcategory',
        headerName: 'Subcategory',
        filter: 'agTextColumnFilter',
        enableRowGroup: true,
        rowGroup: false,
      },
      {
        field: 'price',
        headerName: 'Price',
        filter: 'agNumberColumnFilter',
        valueFormatter: (p) => formatCurrency(p.value),
        type: 'numericColumn',
      },
      {
        field: 'quantity',
        headerName: 'Quantity',
        filter: 'agNumberColumnFilter',
        type: 'numericColumn',
      },
      {
        field: 'rating',
        headerName: 'Rating',
        filter: 'agNumberColumnFilter',
        type: 'numericColumn',
        valueFormatter: (p) => (p.value != null ? Number(p.value).toFixed(1) : ''),
      },
      {
        field: 'created_at',
        headerName: 'Created At',
        valueFormatter: (p) => formatDate(p.value),
        minWidth: 160,
      },
    ],
    [],
  );

  const defaultColDef = useMemo<ColDef>(
    () => ({
      flex: 1,
      minWidth: 120,
      sortable: true,
      resizable: true,
      floatingFilter: true,
    }),
    [],
  );

  const datasource = useCallback((): IServerSideDatasource => {
    return {
      getRows(params: IServerSideGetRowsParams) {
        fetch(`${API_URL}/api/search-products`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(params.request),
        })
          .then((res) => {
            if (!res.ok) throw new Error(`HTTP ${res.status}`);
            return res.json();
          })
          .then((data: { rows: Record<string, unknown>[]; lastRow: number }) => {
            params.success({ rowData: data.rows, rowCount: data.lastRow });
          })
          .catch((err) => {
            console.error('SSRM fetch error:', err);
            params.fail();
          });
      },
    };
  }, []);

  return (
    <div className="ag-theme-alpine" style={{ height: '100%', width: '100%' }}>
      <AgGridReact
        columnDefs={columnDefs}
        defaultColDef={defaultColDef}
        rowModelType="serverSide"
        serverSideDatasource={datasource()}
        rowGroupPanelShow="always"
        groupDisplayType="multipleColumns"
        cacheBlockSize={100}
        maxBlocksInCache={10}
        animateRows={true}
      />
    </div>
  );
}
