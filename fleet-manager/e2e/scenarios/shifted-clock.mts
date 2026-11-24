// The app server of the docs project loads this module with --import. It
// sets the clock of the server to FLEET_CLOCK_START when the server starts,
// and the clock then runs on, so timers, timeouts, and caches keep their
// real durations. The server renders the first ages and times of a page
// from its clock, and the page runs at the same time, so a guide image
// shows the same text on each run.
const start = Date.parse(process.env.FLEET_CLOCK_START ?? "");
if (Number.isNaN(start)) {
  throw new Error("Set FLEET_CLOCK_START to an RFC 3339 time");
}

const RealDate = Date;
const offset = start - RealDate.now();
const now = () => RealDate.now() + offset;

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
