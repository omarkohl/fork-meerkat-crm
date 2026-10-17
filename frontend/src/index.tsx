import React from 'react';
import ReactDOM from 'react-dom/client';
import './index.css';
import App from './App';
import ErrorBoundary from './components/ErrorBoundary';
import './i18n/config';
import { AppThemeProvider } from './AppThemeProvider';
import { DateFormatProvider } from './DateFormatProvider';
import { SnackbarProvider } from './context/SnackbarContext';

const logError = (error: Error, errorInfo: React.ErrorInfo) => {
  console.error('Application Error:', error);
  console.error('Error Info:', errorInfo);
};

const root = ReactDOM.createRoot(document.getElementById('root') as HTMLElement);

root.render(
  <React.StrictMode>
    <AppThemeProvider>
      <DateFormatProvider>
        <SnackbarProvider>
          <ErrorBoundary name="Application" onError={logError} showDetails={import.meta.env.DEV}>
            <App />
          </ErrorBoundary>
        </SnackbarProvider>
      </DateFormatProvider>
    </AppThemeProvider>
  </React.StrictMode>,
);
