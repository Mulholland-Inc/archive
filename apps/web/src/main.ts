import { mount } from 'svelte';
import './app.css';
import App from './App.svelte';
import { loadBrand } from './lib/brand.svelte';

loadBrand();
mount(App, { target: document.getElementById('app')! });
