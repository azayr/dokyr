<script>
  export let value = 'healthy';
  const labels = {
    healthy: 'Healthy',
    ready: 'Ready',
    running: 'Running',
    deploying: 'Deploying',
    building: 'Building',
    queued: 'Queued',
    degraded: 'Degraded',
    failed: 'Failed',
    cancelled: 'Cancelled',
    stopped: 'Stopped'
  };
</script>

<span
  class="status"
  class:good={['healthy', 'ready'].includes(value)}
  class:busy={['deploying', 'building', 'running', 'queued'].includes(value)}
  class:bad={value === 'failed'}
  class:warn={value === 'degraded'}
>
  <i></i>{labels[value] || value}
</span>

<style>
  .status {
    padding: 3px 9px;
    display: inline-flex;
    align-items: center;
    gap: 6px;
    width: max-content;
    max-width: 100%;
    border-radius: 999px;
    background: var(--color-paper-subtle);
    color: var(--color-muted);
    font-size: var(--text-xs);
    font-weight: 600;
    line-height: 1.5;
    white-space: nowrap;
  }
  i {
    width: 6px;
    height: 6px;
    flex: 0 0 auto;
    border-radius: 50%;
    background: currentColor;
  }
  .good {
    background: var(--color-success-soft);
    color: var(--color-success);
  }
  .busy {
    background: var(--color-info-soft);
    color: var(--color-info);
  }
  .busy i {
    animation: dokyr-status-pulse 1.6s ease-out infinite;
  }
  .bad {
    background: var(--color-danger-soft);
    color: var(--color-danger);
  }
  .warn {
    background: var(--color-warning-soft);
    color: var(--color-warning);
  }
  @keyframes dokyr-status-pulse {
    50% {
      box-shadow: 0 0 0 4px color-mix(in srgb, currentColor 12%, transparent);
    }
  }
  @media (prefers-reduced-motion: reduce) {
    .busy i {
      animation: none;
    }
  }
</style>
