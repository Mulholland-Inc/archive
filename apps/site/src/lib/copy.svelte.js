export function clipboard() {
  let copied = $state(false);

  return {
    get copied() {
      return copied;
    },
    async copy(text) {
      await navigator.clipboard.writeText(text);
      copied = true;
      setTimeout(() => (copied = false), 1500);
    }
  };
}
