#ifndef ENGINE_TYPES_H
#define ENGINE_TYPES_H

#ifdef __cplusplus
extern "C" {
#endif

typedef void* engine_handle_t;

typedef enum engine_status_t {
    ENGINE_OK = 0,
    ENGINE_ERR_NULL_HANDLE = 1,
    ENGINE_ERR_INVALID_ARG = 2,
    ENGINE_ERR_NOT_FOUND = 3,
    ENGINE_ERR_INTERNAL = 100
} engine_status_t;

typedef struct {
    int price;
    int volume;
} engine_price_level_t;

typedef struct {
    engine_price_level_t bids[50];
    engine_price_level_t asks[50];
    int bid_count;
    int ask_count;
    int best_bid;
    int best_ask;
    int last_price;
} engine_depth_t;

typedef struct {
    int price;
    int volume;
    int side;
} engine_level_change_t;

typedef struct {
    int price;
    int qty;
    int maker_order_id;
    int taker_order_id;
} engine_trade_t;

typedef struct {
    engine_level_change_t changes[4];
    int change_count;

    engine_trade_t trades[100];
    int trade_count;
} engine_order_result_t;

typedef enum engine_side_t {
    ENGINE_SELL = 0,
    ENGINE_BUY = 1
} engine_side_t;

#ifdef __cplusplus
}
#endif

#endif
