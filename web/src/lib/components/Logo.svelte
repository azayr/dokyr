<script>
  export let size = 28;
  export let wordmark = true;
  export let compact = false;
</script>

<span class="logo" class:compact>
  <svg class="logo-mark" width={size} height={size} viewBox="0 0 512 512" aria-hidden="true">
    <path fill="currentColor" fill-rule="evenodd" d="M 32 39 L 304 40 L 331 45 L 357 54 L 387 70 L 406 84 L 431 109 L 454 143 L 471 184 L 479 225 L 479 270 L 471 310 L 457 345 L 446 364 L 431 384 L 407 408 L 377 429 L 349 442 L 316 451 L 300 453 L 35 453 L 203 365 L 210 358 L 212 348 L 299 348 L 315 342 L 401 266 L 407 256 L 408 242 L 403 231 L 377 206 L 376 206 L 375 206 L 367 197 L 366 197 L 365 197 L 320 156 L 303 147 L 92 147 L 99 154 L 149 191 L 152 197 L 152 235 L 149 234 L 127 216 L 122 214 L 92 190 L 87 188 L 35 149 L 32 141 Z" />
    <path fill="#0d63e5" d="M 209 260 L 212 260 L 212 351 L 209 359 L 204 364 L 192 369 L 191 371 L 32 453 L 32 358 L 34 353 L 53 341 L 81 328 L 82 326 L 129 303 L 130 301 L 154 290 L 155 288 Z" />
  </svg>
  {#if wordmark}<strong>Dokyr</strong>{/if}
</span>

<style>
  .logo {
    display: inline-flex;
    align-items: center;
    gap: 9px;
    color: var(--color-ink);
  }
  .logo svg {
    flex: 0 0 auto;
    display: block;
  }
  .logo-mark {
    color: var(--color-ink);
  }
  .logo strong {
    font-size: 15px;
    font-weight: 700;
    letter-spacing: -0.02em;
  }
  .compact strong {
    display: none;
  }
</style>
