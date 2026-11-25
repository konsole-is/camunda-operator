// The app server of the docs project loads this module with --import. Date
// then reads FLEET_FIXED_NOW at all times, as page.clock.setFixedTime does
// in the page, so a page shows the same ages, times, and names on each run.
// Timers do not use Date, so they keep their real durations.
const fixed = Date.parse(process.env.FLEET_FIXED_NOW ?? "");
if (Number.isNaN(fixed)) {
  throw new Error("Set FLEET_FIXED_NOW to an RFC 3339 time");
}

const RealDate = Date;
const now = () => fixed;

globalThis.Date = new Proxy(RealDate, {
  construct(target, args, newTarget) {
    return Reflect.construct(
      target,
      args.length > 0 ? args : [now()],
      newTarget,
    );
  },
  // Date() without new returns the time as a string.
  apply() {
    return new RealDate(now()).toString();
  },
  get(target, key, receiver) {
    return key === "now" ? now : Reflect.get(target, key, receiver);
  },
});
