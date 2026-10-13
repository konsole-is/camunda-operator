/** Logo is the orange squared [C] mark and the name of the app. */
export function Logo() {
  return (
    <span className="flex items-center gap-3">
      <span className="flex size-9 items-center justify-center rounded-[9px] border border-accent bg-panel-2 shadow-[0_0_18px_rgba(255,107,26,0.35)]">
        <svg
          width="22"
          height="22"
          viewBox="0 0 32 32"
          fill="none"
          aria-hidden="true"
        >
          <path
            d="M25 7H13a6 6 0 0 0-6 6v6a6 6 0 0 0 6 6h12"
            stroke="var(--accent)"
            strokeWidth="5.5"
            strokeLinecap="square"
          />
        </svg>
      </span>
      <span className="font-heading text-[17px] font-bold tracking-[-0.01em] text-text">
        Fleet manager
      </span>
    </span>
  );
}
