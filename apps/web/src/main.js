import { mount } from 'svelte';
import '#kit/tokens.css';
import '#kit/ui.css';
import './base.css';
import App from './App.svelte';

mount(App, { target: document.getElementById('app') });
