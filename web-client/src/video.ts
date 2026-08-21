import { LIMITS } from "./limits";
import type { EncodedVideoFrames } from "./generated/kessoku_wire";

const VP9_CODEC = "vp09.00.10.08";

export function webCodecsTimestamp(ptsMilliseconds: bigint): number {
  if (ptsMilliseconds < 0n || ptsMilliseconds > BigInt(Math.floor(Number.MAX_SAFE_INTEGER / 1000))) {
    throw new Error("Invalid VP9 timestamp");
  }
  return Number(ptsMilliseconds) * 1000;
}

export class Vp9CanvasRenderer {
  readonly #canvas: HTMLCanvasElement;
  readonly #context: CanvasRenderingContext2D;
  readonly #decoder: VideoDecoder;
  readonly #acknowledge: () => void;
  readonly #fail: (error: Error) => void;
  #closed = false;
  #pendingFrames = 0;
  readonly #pendingBatches: number[] = [];

  private constructor(canvas: HTMLCanvasElement, acknowledge: () => void, fail: (error: Error) => void) {
    const context = canvas.getContext("2d", { alpha: false });
    if (context === null) throw new Error("Canvas 2D is unavailable");
    this.#canvas = canvas;
    this.#context = context;
    this.#acknowledge = acknowledge;
    this.#fail = fail;
    this.#decoder = new VideoDecoder({
      output: (frame) => this.#draw(frame),
      error: (error) => this.#fail(error instanceof Error ? error : new Error("VP9 decoder failed")),
    });
    this.#decoder.configure({ codec: VP9_CODEC, optimizeForLatency: true, hardwareAcceleration: "no-preference" });
  }

  static async create(canvas: HTMLCanvasElement, acknowledge: () => void, fail: (error: Error) => void): Promise<Vp9CanvasRenderer> {
    if (!("VideoDecoder" in globalThis) || !("EncodedVideoChunk" in globalThis)) throw new Error("VP9 WebCodecs is unavailable");
    const support = await VideoDecoder.isConfigSupported({ codec: VP9_CODEC, optimizeForLatency: true, hardwareAcceleration: "no-preference" });
    if (!support.supported) throw new Error("This browser cannot decode VP9 with WebCodecs");
    return new Vp9CanvasRenderer(canvas, acknowledge, fail);
  }

  setDisplay(width: number, height: number): void {
    if (width <= 0 || height <= 0 || width > LIMITS.displayDimension || height > LIMITS.displayDimension || width * height > LIMITS.displayPixels) {
      throw new Error("Display dimensions exceed client limits");
    }
    this.#canvas.width = width;
    this.#canvas.height = height;
  }

  decode(batch: EncodedVideoFrames): void {
    if (this.#closed) throw new Error("Video decoder is closed");
    if (batch.frames.length === 0 || batch.frames.length > LIMITS.queuedVideoChunks) throw new Error("Invalid VP9 frame batch");
    if (this.#pendingFrames + batch.frames.length > LIMITS.queuedVideoChunks) throw new Error("VP9 decode queue limit exceeded");
    this.#pendingFrames += batch.frames.length;
    this.#pendingBatches.push(batch.frames.length);
    for (const frame of batch.frames) {
      if (frame.data.byteLength === 0 || frame.data.byteLength > LIMITS.encodedVideoChunk) throw new Error("Encoded VP9 frame exceeds limits");
      this.#decoder.decode(new EncodedVideoChunk({
        type: frame.key ? "key" : "delta",
        timestamp: webCodecsTimestamp(frame.pts),
        data: frame.data,
      }));
    }
  }

  #draw(frame: globalThis.VideoFrame): void {
    try {
      if (this.#closed || frame.codedWidth <= 0 || frame.codedHeight <= 0 ||
        frame.codedWidth > LIMITS.displayDimension || frame.codedHeight > LIMITS.displayDimension ||
        frame.codedWidth * frame.codedHeight > LIMITS.displayPixels) {
        throw new Error("Decoded VP9 frame exceeds limits");
      }
      this.#context.drawImage(frame, 0, 0, this.#canvas.width, this.#canvas.height);
      this.#pendingFrames -= 1;
      const remaining = this.#pendingBatches[0];
      if (remaining === undefined) throw new Error("VP9 decoder produced an unexpected frame");
      if (remaining === 1) {
        this.#pendingBatches.shift();
        this.#acknowledge();
      } else {
        this.#pendingBatches[0] = remaining - 1;
      }
    } catch (error) {
      this.#fail(error instanceof Error ? error : new Error("Unable to draw VP9 frame"));
    } finally {
      frame.close();
    }
  }

  close(): void {
    if (this.#closed) return;
    this.#closed = true;
    this.#decoder.close();
    this.#pendingFrames = 0;
    this.#pendingBatches.length = 0;
    this.#context.clearRect(0, 0, this.#canvas.width, this.#canvas.height);
  }
}
