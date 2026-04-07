import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { ModuleRegistry } from 'ag-grid-community';
import { AllEnterpriseModule } from 'ag-grid-enterprise';
import './index.css';
import App from './App.tsx';

ModuleRegistry.registerModules([AllEnterpriseModule]);

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
