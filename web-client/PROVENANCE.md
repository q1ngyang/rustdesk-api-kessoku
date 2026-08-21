# Provenance

All handwritten HTML, CSS, and TypeScript in this directory was authored for this repository from the normative inputs below:

- `../docs/development/WEB-CLIENT-WIRE-SPEC.md`
- `proto/kessoku_wire.proto`
- the frozen Kessoku `/config/v1.json` and authentication API contracts

Generated file:

- `src/generated/kessoku_wire.ts`: `protoc` 28.3 with `ts-proto` 2.12.1, using the command in `package.json`.

Runtime dependency:

- `@bufbuild/protobuf` 2.9.0, used only for protobuf wire reading and writing; licensed under `(Apache-2.0 AND BSD-3-Clause)`. Its complete applicable license and attribution text is preserved at `public/third-party-licenses/@bufbuild-protobuf-2.9.0.txt` and copied by Vite into every production build.

Development dependencies are pinned exactly in `package.json` and transitively in `package-lock.json`. The application contains no remote scripts, analytics, external fonts, JavaScript/WASM crypto bundle, copied web client, or service worker.
