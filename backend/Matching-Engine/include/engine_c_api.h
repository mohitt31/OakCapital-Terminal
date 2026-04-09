#ifndef ENGINE_C_API_H
#define ENGINE_C_API_H

#include "engine_types.h"

#ifdef __cplusplus
extern "C" {
#endif

engine_handle_t engine_create(void);
void engine_destroy(engine_handle_t handle);

// Limit orders
engine_status_t engine_add_limit(engine_handle_t handle, int order_id, int side, int qty, int limit_price, engine_order_result_t* result);
engine_status_t engine_market(engine_handle_t handle, int order_id, int side, int qty, engine_order_result_t* result);
engine_status_t engine_cancel_limit(engine_handle_t handle, int order_id, engine_order_result_t* result);
engine_status_t engine_modify_limit(engine_handle_t handle, int order_id, int new_qty, int new_limit, engine_order_result_t* result);

// Stop orders
engine_status_t engine_add_stop(engine_handle_t handle, int order_id, int side, int qty, int stop_price, engine_order_result_t* result);
engine_status_t engine_cancel_stop(engine_handle_t handle, int order_id, engine_order_result_t* result);
engine_status_t engine_modify_stop(engine_handle_t handle, int order_id, int new_qty, int new_stop_price, engine_order_result_t* result);

// Stop-limit orders
engine_status_t engine_add_stop_limit(engine_handle_t handle, int order_id, int side, int qty, int limit_price, int stop_price, engine_order_result_t* result);
engine_status_t engine_cancel_stop_limit(engine_handle_t handle, int order_id, engine_order_result_t* result);
engine_status_t engine_modify_stop_limit(engine_handle_t handle, int order_id, int new_qty, int new_limit_price, int new_stop_price, engine_order_result_t* result);

// Queries
int engine_best_bid(engine_handle_t handle);
int engine_best_ask(engine_handle_t handle);
int engine_last_executed_count(engine_handle_t handle);
int engine_last_executed_price(engine_handle_t handle);
void engine_get_depth(engine_handle_t handle, engine_depth_t* out);

#ifdef __cplusplus
}
#endif

#endif
