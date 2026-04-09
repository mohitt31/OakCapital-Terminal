#include "engine_c_api.h"

#include "Book.hpp"
#include "Limit.hpp"
#include "Order.hpp"

#include <algorithm>
#include <cstring>

namespace {

Book* as_book(engine_handle_t handle) {
    return reinterpret_cast<Book*>(handle);
}

bool is_valid_side(int side) {
    return side == ENGINE_BUY || side == ENGINE_SELL;
}

bool is_positive(int value) {
    return value > 0;
}

} // namespace

extern "C" {

static void capture_level(Book* b, engine_order_result_t* result, int price, bool isBuy) {
    if (price <= 0 || !result) return;
    int idx = result->change_count;
    if (idx >= 4) return;

    Limit* level = b->searchLimitMaps(price, isBuy);
    result->changes[idx].price  = price;
    result->changes[idx].volume = level ? level->getTotalVolume() : 0;
    result->changes[idx].side   = isBuy ? ENGINE_BUY : ENGINE_SELL;
    result->change_count++;
}

static void capture_trades(Book* b, engine_order_result_t* result) {
    if (!result || b->lastTrades.empty()) return;
    int count = std::min((int)b->lastTrades.size(), 100);
    for (int i = 0; i < count; i++) {
        result->trades[i].price = b->lastTrades[i].price;
        result->trades[i].qty = b->lastTrades[i].qty;
        result->trades[i].maker_order_id = b->lastTrades[i].makerOrderId;
        result->trades[i].taker_order_id = b->lastTrades[i].takerOrderId;
    }
    result->trade_count = count;
}

engine_handle_t engine_create(void) {
    try {
        return reinterpret_cast<engine_handle_t>(new Book());
    } catch (...) {
        return nullptr;
    }
}

void engine_destroy(engine_handle_t handle) {
    delete as_book(handle);
}

engine_status_t engine_add_limit(engine_handle_t handle, int order_id, int side,
                                  int qty, int limit_price, engine_order_result_t* result) {
    if (!handle) return ENGINE_ERR_NULL_HANDLE;
    if (!is_valid_side(side) || !is_positive(qty) || !is_positive(limit_price))
        return ENGINE_ERR_INVALID_ARG;

    try {
        Book* b = as_book(handle);

        int prevBestBid = b->getHighestBuy() ? b->getHighestBuy()->getLimitPrice() : -1;
        int prevBestAsk = b->getLowestSell() ? b->getLowestSell()->getLimitPrice() : -1;

        if (result) memset(result, 0, sizeof(engine_order_result_t));

        b->addLimitOrder(order_id, side == ENGINE_BUY, qty, limit_price);

        if (result) {
            if (b->executedOrdersCount > 0) {
                capture_trades(b, result);
            }

            capture_level(b, result, limit_price, side == ENGINE_BUY);

            if (side == ENGINE_BUY && prevBestAsk > 0)
                capture_level(b, result, prevBestAsk, false);
            else if (side == ENGINE_SELL && prevBestBid > 0)
                capture_level(b, result, prevBestBid, true);
        }
        return ENGINE_OK;
    } catch (...) { return ENGINE_ERR_INTERNAL; }
}

engine_status_t engine_market(engine_handle_t handle, int order_id, int side,
                               int qty, engine_order_result_t* result) {
    if (!handle) return ENGINE_ERR_NULL_HANDLE;
    if (!is_valid_side(side) || !is_positive(qty)) return ENGINE_ERR_INVALID_ARG;

    try {
        Book* b = as_book(handle);
        int prevBestBid = b->getHighestBuy() ? b->getHighestBuy()->getLimitPrice() : -1;
        int prevBestAsk = b->getLowestSell() ? b->getLowestSell()->getLimitPrice() : -1;

        if (result) memset(result, 0, sizeof(engine_order_result_t));

        b->marketOrder(order_id, side == ENGINE_BUY, qty);

        if (result) {
            if (b->executedOrdersCount > 0) {
                capture_trades(b, result);
            }

            if (side == ENGINE_BUY)
                capture_level(b, result, prevBestAsk, false);
            else
                capture_level(b, result, prevBestBid, true);

            Limit* newEdge = side == ENGINE_BUY ? b->getLowestSell() : b->getHighestBuy();
            if (newEdge)
                capture_level(b, result, newEdge->getLimitPrice(), side != ENGINE_BUY);
        }
        return ENGINE_OK;
    } catch (...) { return ENGINE_ERR_INTERNAL; }
}

engine_status_t engine_cancel_limit(engine_handle_t handle, int order_id,
                                     engine_order_result_t* result) {
    if (!handle) return ENGINE_ERR_NULL_HANDLE;

    try {
        Book* b = as_book(handle);

        Order* order = b->searchOrderMap(order_id);
        if (!order) return ENGINE_ERR_NOT_FOUND;

        int cancelPrice = order->getLimit();
        bool cancelSide = order->getBuyOrSell();

        if (result) memset(result, 0, sizeof(engine_order_result_t));

        b->cancelLimitOrder(order_id);

        if (result) {
            capture_level(b, result, cancelPrice, cancelSide);
        }
        return ENGINE_OK;
    } catch (...) { return ENGINE_ERR_INTERNAL; }
}

engine_status_t engine_modify_limit(engine_handle_t handle, int order_id,
                                     int new_qty, int new_limit,
                                     engine_order_result_t* result) {
    if (!handle) return ENGINE_ERR_NULL_HANDLE;
    if (!is_positive(new_qty) || !is_positive(new_limit)) return ENGINE_ERR_INVALID_ARG;

    try {
        Book* b = as_book(handle);

        Order* order = b->searchOrderMap(order_id);
        if (!order) return ENGINE_ERR_NOT_FOUND;

        int oldPrice = order->getLimit();
        bool side    = order->getBuyOrSell();

        if (result) memset(result, 0, sizeof(engine_order_result_t));

        b->modifyLimitOrder(order_id, new_qty, new_limit);

        if (result) {
            capture_level(b, result, oldPrice, side);
            if (new_limit != oldPrice)
                capture_level(b, result, new_limit, side);

            if (b->executedOrdersCount > 0) {
                capture_trades(b, result);
            }
        }
        return ENGINE_OK;
    } catch (...) { return ENGINE_ERR_INTERNAL; }
}

engine_status_t engine_add_stop(engine_handle_t handle, int order_id, int side,
                                 int qty, int stop_price,
                                 engine_order_result_t* result) {
    if (!handle) return ENGINE_ERR_NULL_HANDLE;
    if (!is_valid_side(side) || !is_positive(qty) || !is_positive(stop_price))
        return ENGINE_ERR_INVALID_ARG;

    try {
        Book* b = as_book(handle);

        int prevExecCount = b->executedOrdersCount;
        int prevBestBid = b->getHighestBuy() ? b->getHighestBuy()->getLimitPrice() : -1;
        int prevBestAsk = b->getLowestSell() ? b->getLowestSell()->getLimitPrice() : -1;

        if (result) memset(result, 0, sizeof(engine_order_result_t));

        b->addStopOrder(order_id, side == ENGINE_BUY, qty, stop_price);

        if (result) {
            if (b->executedOrdersCount > 0) {
                capture_trades(b, result);

                if (side == ENGINE_BUY && prevBestAsk > 0)
                    capture_level(b, result, prevBestAsk, false);
                else if (side == ENGINE_SELL && prevBestBid > 0)
                    capture_level(b, result, prevBestBid, true);

                Limit* newEdge = side == ENGINE_BUY
                    ? b->getLowestSell() : b->getHighestBuy();
                if (newEdge)
                    capture_level(b, result, newEdge->getLimitPrice(), side != ENGINE_BUY);
            }
        }
        return ENGINE_OK;
    } catch (...) { return ENGINE_ERR_INTERNAL; }
}

engine_status_t engine_cancel_stop(engine_handle_t handle, int order_id,
                                    engine_order_result_t* result) {
    if (!handle) return ENGINE_ERR_NULL_HANDLE;

    try {
        Book* b = as_book(handle);

        if (b->searchOrderMap(order_id) == nullptr)
            return ENGINE_ERR_NOT_FOUND;

        if (result) memset(result, 0, sizeof(engine_order_result_t));

        b->cancelStopOrder(order_id);

        return ENGINE_OK;
    } catch (...) { return ENGINE_ERR_INTERNAL; }
}

engine_status_t engine_modify_stop(engine_handle_t handle, int order_id,
                                    int new_qty, int new_stop_price,
                                    engine_order_result_t* result) {
    if (!handle) return ENGINE_ERR_NULL_HANDLE;
    if (!is_positive(new_qty) || !is_positive(new_stop_price))
        return ENGINE_ERR_INVALID_ARG;

    try {
        Book* b = as_book(handle);

        if (b->searchOrderMap(order_id) == nullptr)
            return ENGINE_ERR_NOT_FOUND;

        int prevBestBid = b->getHighestBuy() ? b->getHighestBuy()->getLimitPrice() : -1;
        int prevBestAsk = b->getLowestSell() ? b->getLowestSell()->getLimitPrice() : -1;

        if (result) memset(result, 0, sizeof(engine_order_result_t));

        b->modifyStopOrder(order_id, new_qty, new_stop_price);

        if (result && b->executedOrdersCount > 0) {
            capture_trades(b, result);

            if (prevBestAsk > 0) capture_level(b, result, prevBestAsk, false);
            if (prevBestBid > 0) capture_level(b, result, prevBestBid, true);
        }
        return ENGINE_OK;
    } catch (...) { return ENGINE_ERR_INTERNAL; }
}

engine_status_t engine_add_stop_limit(engine_handle_t handle, int order_id, int side,
                                       int qty, int limit_price, int stop_price,
                                       engine_order_result_t* result) {
    if (!handle) return ENGINE_ERR_NULL_HANDLE;
    if (!is_valid_side(side) || !is_positive(qty) ||
        !is_positive(limit_price) || !is_positive(stop_price))
        return ENGINE_ERR_INVALID_ARG;

    try {
        Book* b = as_book(handle);

        int prevBestBid = b->getHighestBuy() ? b->getHighestBuy()->getLimitPrice() : -1;
        int prevBestAsk = b->getLowestSell() ? b->getLowestSell()->getLimitPrice() : -1;

        if (result) memset(result, 0, sizeof(engine_order_result_t));

        b->addStopLimitOrder(order_id, side == ENGINE_BUY, qty, limit_price, stop_price);

        if (result) {
            if (b->executedOrdersCount > 0) {
                capture_trades(b, result);

                if (side == ENGINE_BUY && prevBestAsk > 0)
                    capture_level(b, result, prevBestAsk, false);
                else if (side == ENGINE_SELL && prevBestBid > 0)
                    capture_level(b, result, prevBestBid, true);
            } else {
                capture_level(b, result, limit_price, side == ENGINE_BUY);
            }
        }
        return ENGINE_OK;
    } catch (...) { return ENGINE_ERR_INTERNAL; }
}

engine_status_t engine_cancel_stop_limit(engine_handle_t handle, int order_id,
                                          engine_order_result_t* result) {
    if (!handle) return ENGINE_ERR_NULL_HANDLE;

    try {
        Book* b = as_book(handle);

        Order* order = b->searchOrderMap(order_id);
        if (!order) return ENGINE_ERR_NOT_FOUND;

        int limitPrice = order->getLimit();
        bool side      = order->getBuyOrSell();
        bool isInLimitBook = (limitPrice > 0);

        if (result) memset(result, 0, sizeof(engine_order_result_t));

        b->cancelStopLimitOrder(order_id);

        if (result && isInLimitBook) {
            capture_level(b, result, limitPrice, side);
        }
        return ENGINE_OK;
    } catch (...) { return ENGINE_ERR_INTERNAL; }
}

engine_status_t engine_modify_stop_limit(engine_handle_t handle, int order_id,
                                          int new_qty, int new_limit_price,
                                          int new_stop_price,
                                          engine_order_result_t* result) {
    if (!handle) return ENGINE_ERR_NULL_HANDLE;
    if (!is_positive(new_qty) || !is_positive(new_limit_price) ||
        !is_positive(new_stop_price))
        return ENGINE_ERR_INVALID_ARG;

    try {
        Book* b = as_book(handle);

        Order* order = b->searchOrderMap(order_id);
        if (!order) return ENGINE_ERR_NOT_FOUND;

        int oldLimit   = order->getLimit();
        bool side      = order->getBuyOrSell();

        if (result) memset(result, 0, sizeof(engine_order_result_t));

        b->modifyStopLimitOrder(order_id, new_qty, new_limit_price, new_stop_price);

        if (result) {
            if (b->executedOrdersCount > 0) {
                capture_trades(b, result);
            }
            if (oldLimit > 0)
                capture_level(b, result, oldLimit, side);
            if (new_limit_price != oldLimit)
                capture_level(b, result, new_limit_price, side);
        }
        return ENGINE_OK;
    } catch (...) { return ENGINE_ERR_INTERNAL; }
}

int engine_best_bid(engine_handle_t handle) {
    if (!handle) return -1;
    Limit* level = as_book(handle)->getHighestBuy();
    return level ? level->getLimitPrice() : -1;
}

int engine_best_ask(engine_handle_t handle) {
    if (!handle) return -1;
    Limit* level = as_book(handle)->getLowestSell();
    return level ? level->getLimitPrice() : -1;
}

int engine_last_executed_count(engine_handle_t handle) {
    if (!handle) return -1;
    return as_book(handle)->executedOrdersCount;
}

int engine_last_executed_price(engine_handle_t handle) {
    if (!handle) return -1;
    return as_book(handle)->lastExecutedPrice;
}

void engine_get_depth(engine_handle_t handle, engine_depth_t* out) {
    if (!handle || !out) return;
    memset(out, 0, sizeof(engine_depth_t));

    Book* b = as_book(handle);

    Limit* highBuy = b->getHighestBuy();
    Limit* lowSell = b->getLowestSell();
    out->best_bid   = highBuy ? highBuy->getLimitPrice() : -1;
    out->best_ask   = lowSell ? lowSell->getLimitPrice() : -1;
    out->last_price = b->lastExecutedPrice;

    std::vector<int> bidPrices = b->inOrderTreeTraversal(b->getBuyTree());
    std::reverse(bidPrices.begin(), bidPrices.end());
    int bidCount = std::min((int)bidPrices.size(), 50);
    for (int i = 0; i < bidCount; i++) {
        Limit* level = b->searchLimitMaps(bidPrices[i], true);
        if (level) {
            out->bids[i].price  = bidPrices[i];
            out->bids[i].volume = level->getTotalVolume();
        }
    }
    out->bid_count = bidCount;

    std::vector<int> askPrices = b->inOrderTreeTraversal(b->getSellTree());
    int askCount = std::min((int)askPrices.size(), 50);
    for (int i = 0; i < askCount; i++) {
        Limit* level = b->searchLimitMaps(askPrices[i], false);
        if (level) {
            out->asks[i].price  = askPrices[i];
            out->asks[i].volume = level->getTotalVolume();
        }
    }
    out->ask_count = askCount;
}

} // extern "C"
