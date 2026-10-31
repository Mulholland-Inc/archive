import { mount } from 'svelte';
import '@fontsource-variable/inter';
import '@fontsource-variable/inter/wght-italic.css';
import '@fontsource-variable/eb-garamond';
import '@fontsource-variable/eb-garamond/wght-italic.css';
import '@fontsource-variable/geist-mono';
import './styles/tokens.css';
import './styles/base.css';
import './styles/bots.css';
import './lib/theme.svelte.js';
import App from './App.svelte';

export default mount(App, { target: document.getElementById('app') });
