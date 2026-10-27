import { mount } from 'svelte';
import './tokens.css';
import './base.css';
import App from './App.svelte';

mount(App, { target: document.getElementById('app') });
