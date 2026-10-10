// Extends Vitest's expect with jest-dom matchers (toBeInTheDocument, etc.)
import '@testing-library/jest-dom/vitest';
import { cleanup } from '@testing-library/react';
import { afterEach } from 'vitest';

// jsdom does not implement scrolling; App calls window.scrollTo on route changes
window.scrollTo = () => {};

// Vitest runs without globals, so Testing Library does not register its automatic
// cleanup. Without it every rendered tree stays mounted after its test, and pending
// react-transition-group timers (MUI dialogs, snackbars) can fire after the jsdom
// environment is torn down ("window is not defined", exit code 1).
afterEach(cleanup);
