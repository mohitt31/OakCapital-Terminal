# Stage 1: Build the C++ matching engine shared library
FROM alpine:3.21 AS cpp-builder

RUN apk add --no-cache build-base cmake

WORKDIR /build

COPY Matching-Engine/ ./Matching-Engine/

RUN cd Matching-Engine && \
    cmake -B build -DMATCHING_ENGINE_BUILD_SMOKE=OFF && \
    cmake --build build --target matching_engine_c_api -j"$(nproc)"

# Stage 2: Build the Go binary with CGO linking to the C++ engine
FROM golang:1.25-alpine AS go-builder

RUN apk add --no-cache build-base

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

COPY --from=cpp-builder /build/Matching-Engine/build/libmatching_engine_c_api.so ./lib/

RUN CGO_ENABLED=1 go build -o /app/server ./cmd/server
RUN CGO_ENABLED=1 go build -o /app/mmbot ./cmd/mmbot

# Stage 3: Minimal runtime image
FROM alpine:3.21

RUN apk add --no-cache ca-certificates libstdc++ libgcc

WORKDIR /app

COPY --from=go-builder /app/server .
COPY --from=go-builder /app/mmbot .
COPY --from=cpp-builder /build/Matching-Engine/build/libmatching_engine_c_api.so /usr/local/lib/
COPY internal/db/migrations/ ./internal/db/migrations/

EXPOSE 8080

CMD ["./server"]
