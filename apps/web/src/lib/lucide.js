const icons = import.meta.glob(['../../node_modules/@lucide/svelte/dist/icons/*.js', '!../../node_modules/@lucide/svelte/dist/icons/index.js']);

export const load = async (name) => (await icons[`../../node_modules/@lucide/svelte/dist/icons/${name}.js`]()).default;
