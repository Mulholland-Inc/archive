import { mount } from 'svelte';
import '@fontsource-variable/inter';
import '@fontsource-variable/inter/wght-italic.css';
import '@fontsource-variable/eb-garamond';
import '@fontsource-variable/eb-garamond/wght-italic.css';
import './styles/tokens.css';
import './styles/base.css';
import './styles/bots.css';
import './lib/theme.svelte.js';
import './lib/viewport.js';
import App from './App.svelte';
import Authorize from './pages/Authorize.svelte';
import './lib/preload.js';

// /authorize is an MCP client asking for an archive (pages/Authorize.svelte): it signs in and picks one, and
// starts nothing of the app's own.
export default mount(location.pathname === '/authorize' ? Authorize : App, { target: document.getElementById('app') });
