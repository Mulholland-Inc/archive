<script>
  import Art from './Art.svelte';
  import { reveal } from './reveal.js';

  let { post, lead = false } = $props();
</script>

<a class={[lead ? 'blog-lead' : ['blog-item', post.layout], 'reveal']} href="/blog/{post.slug}" use:reveal>
  <Art name="blog-{post.slug}" width={post.size[0]} height={post.size[1]} lazy={!lead} />
  <h2>{post.title}</h2>
  <p>{post.summary}</p>
</a>
