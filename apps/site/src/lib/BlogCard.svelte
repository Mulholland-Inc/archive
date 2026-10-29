<script>
  import Art from './Art.svelte';
  import { reveal } from './reveal.js';

  let { post, lead = false } = $props();
</script>

<a class={[lead ? 'blog-lead' : 'blog-item', 'reveal']} href="/blog/{post.slug}" use:reveal>
  <div class="blog-art"><Art name="blog-{post.slug}" alt={post.alt} width={post.size[0]} height={post.size[1]} lazy={!lead} /></div>
  <div class="blog-copy">
    <h2>{post.title}</h2>
    <p>{post.summary}</p>
  </div>
</a>
