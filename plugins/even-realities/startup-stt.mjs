// Start once per plugin launch, after settings, authentication and HUD setup.
// Later status refreshes must never undo a user's stop or retry a failed start.
export class StartupSTT {
  constructor({state, start}) { this.state = state; this.start = start; this.pending = true; }
  cancel() { this.pending = false; }
  async attempt() {
    if (!this.pending) return false;
    const {connected, restored, authenticated, busy} = this.state();
    if (!connected || !restored || !authenticated) return false;
    this.pending = false;
    if (busy) return false;
    await this.start();
    return true;
  }
}
