import { mount } from 'svelte';
import '@fontsource-variable/inter';
import '@fontsource-variable/inter/wght-italic.css';
import '@fontsource-variable/eb-garamond';
import '@fontsource-variable/eb-garamond/wght-italic.css';
import './styles/tokens.css';
import './styles/base.css';
import './lib/theme.svelte.js';
import './lib/viewport.js';
import App from './App.svelte';
import './lib/preload.js';

const target = document.getElementById('app');
if (location.pathname === '/authorize') import('./pages/Authorize.svelte').then((m) => mount(m.default, { target }));
else mount(App, { target });
