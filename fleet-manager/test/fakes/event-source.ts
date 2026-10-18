import { act } from "@testing-library/react";

// FakeEventSource stands in for the browser EventSource, so a test can send
// the frames of /api/watch. A test stubs the global EventSource with it and
// resets instances before each test.
export class FakeEventSource {
  static instances: FakeEventSource[] = [];
  static readonly CLOSED = 2;
  readyState = 0;
  closed = false;
  private readonly listeners = new Map<string, ((event: Event) => void)[]>();

  constructor(readonly url: string) {
    FakeEventSource.instances.push(this);
  }

  addEventListener(type: string, listener: (event: Event) => void) {
    this.listeners.set(type, [...(this.listeners.get(type) ?? []), listener]);
  }

  close() {
    this.closed = true;
    this.readyState = FakeEventSource.CLOSED;
  }

  /** emit sends one frame of the given event with data as its JSON. */
  emit(type: string, data: unknown) {
    this.dispatch(new MessageEvent(type, { data: JSON.stringify(data) }));
  }

  /**
   * fail sends an error event as the browser does: readyState 0 when it
   * connects again by itself, 2 when it gave up.
   */
  fail(readyState: 0 | 2) {
    this.readyState = readyState;
    this.dispatch(new Event("error"));
  }

  /** ids returns the values of the parameter watch of the URL. */
  ids(): string[] {
    return new URL(this.url, "http://localhost").searchParams.getAll("watch");
  }

  private dispatch(event: Event) {
    act(() => this.listeners.get(event.type)?.forEach((l) => l(event)));
  }
}
