FROM debian:bookworm-slim@sha256:abd67ffcfa541b485a3dff59865ab629aa048a6c613e639d36e7456b0b229241

WORKDIR /app
COPY ./release /app/
RUN test ! -e /app/resources/web && \
    test ! -e /app/resources/web2 && \
    test -s /app/resources/admin/index.html && \
    test -s /app/resources/client/index.html && \
    test -s /app/resources/client/third-party-licenses/@bufbuild-protobuf-2.9.0.txt && \
    mkdir -p /app/data /app/runtime && \
    chown -R 65534:65534 /app/data /app/runtime

USER 65534:65534
VOLUME /app/data
EXPOSE 21114 21121 21122
CMD ["./kessoku-api"]
