import ProductGrid from './components/ProductGrid';

export default function App() {
  return (
    <div style={{ height: '100vh', width: '100vw', display: 'flex', flexDirection: 'column' }}>
      <header
        style={{
          padding: '12px 20px',
          background: '#1e40af',
          color: '#fff',
          fontWeight: 600,
          fontSize: '1.1rem',
          flexShrink: 0,
        }}
      >
        AG-Grid SSRM — Products POC
      </header>
      <div style={{ flex: 1, overflow: 'hidden' }}>
        <ProductGrid />
      </div>
    </div>
  );
}
