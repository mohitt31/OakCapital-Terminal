#include <ctime>
#include <iomanip>
#include <iostream>
#include <mutex>
#include <sstream>
#include <string>
#include <thread>
#include <unordered_map>

// cpp-httplib header - no SSL/TLS required for basic HTTP
#include <httplib.h>
#include <nlohmann/json.hpp>

// Local includes
#include "../internal/Book.hpp"
#include "../internal/Limit.hpp"
#include "../internal/Order.hpp"

using json = nlohmann::json;

/**
 * HTTP Server for Limit Order Book (Matching Engine)
 * Handles multiple stocks/options with separate order books
 */

class OrderBookServer
{
private:
  httplib::Server server;
  std::unordered_map<std::string, Book *> orderBooks;
  std::mutex booksMutex;

  static constexpr int PORT = 8080;
  static constexpr const char *HOST = "0.0.0.0";

public:
  OrderBookServer() { setupRoutes(); }

  ~OrderBookServer()
  {
    for (auto &[symbol, book] : orderBooks)
    {
      delete book;
    }
    orderBooks.clear();
  }

  void setupRoutes()
  {
    server.Get("/health", [](const httplib::Request &, httplib::Response &res)
               {
      json response = {{"status", "ok"}};
      res.set_content(response.dump(), "application/json"); });

    server.Post("/book/:symbol", [this](const httplib::Request &req,
                                        httplib::Response &res)
                {
      std::string symbol(req.path_params.at("symbol"));

      std::lock_guard<std::mutex> lock(booksMutex);
      if (orderBooks.find(symbol) == orderBooks.end()) {
        orderBooks[symbol] = new Book();
        json response = {{"status", "created"}, {"symbol", symbol}};
        res.set_content(response.dump(), "application/json");
      } else {
        res.status = 400;
        json response = {{"error", "Order book already exists for symbol"}};
        res.set_content(response.dump(), "application/json");
      } });

    server.Post("/order/limit", [this](const httplib::Request &req,
                                       httplib::Response &res)
                {
      try {
        json body;
        if (!parseJsonBody(req, body, res)) {
          return;
        }

        if (!body.contains("symbol") || !body["symbol"].is_string() ||
            !body.contains("orderId") || !body["orderId"].is_number_integer() ||
            !body.contains("buyOrSell") || !body["buyOrSell"].is_boolean() ||
            !body.contains("shares") || !body["shares"].is_number_integer() ||
            !body.contains("limitPrice") ||
            !body["limitPrice"].is_number_integer()) {
          res.status = 400;
          json response = {{"error", "Invalid or missing fields"}};
          res.set_content(response.dump(), "application/json");
          return;
        }

        std::string symbol = body["symbol"].get<std::string>();
        int orderId = body["orderId"].get<int>();
        bool buyOrSell = body["buyOrSell"].get<bool>();
        int shares = body["shares"].get<int>();
        int limitPrice = body["limitPrice"].get<int>();

        if (symbol.empty() || shares <= 0 || limitPrice <= 0) {
          res.status = 400;
          json response = {{"error", "Invalid input parameters"}};
          res.set_content(response.dump(), "application/json");
          return;
        }

        std::lock_guard<std::mutex> lock(booksMutex);

        if (orderBooks.find(symbol) == orderBooks.end()) {
          orderBooks[symbol] = new Book();
        }

        orderBooks[symbol]->addLimitOrder(orderId, buyOrSell, shares,
                                          limitPrice);

        json response = {{"status", "order_added"},
                         {"orderId", orderId},
                         {"symbol", symbol}};
        res.set_content(response.dump(), "application/json");
      } catch (const std::exception &e) {
        res.status = 500;
        json response = {{"error", e.what()}};
        res.set_content(response.dump(), "application/json");
      } });

    server.Post("/order/market", [this](const httplib::Request &req,
                                        httplib::Response &res)
                {
      try {
        json body;
        if (!parseJsonBody(req, body, res)) {
          return;
        }

        if (!body.contains("symbol") || !body["symbol"].is_string() ||
            !body.contains("orderId") || !body["orderId"].is_number_integer() ||
            !body.contains("buyOrSell") || !body["buyOrSell"].is_boolean() ||
            !body.contains("shares") || !body["shares"].is_number_integer()) {
          res.status = 400;
          json response = {{"error", "Invalid or missing fields"}};
          res.set_content(response.dump(), "application/json");
          return;
        }

        std::string symbol = body["symbol"].get<std::string>();
        int orderId = body["orderId"].get<int>();
        bool buyOrSell = body["buyOrSell"].get<bool>();
        int shares = body["shares"].get<int>();

        if (symbol.empty() || shares <= 0) {
          res.status = 400;
          json response = {{"error", "Invalid input parameters"}};
          res.set_content(response.dump(), "application/json");
          return;
        }

        std::lock_guard<std::mutex> lock(booksMutex);

        if (orderBooks.find(symbol) == orderBooks.end()) {
          orderBooks[symbol] = new Book();
        }

        orderBooks[symbol]->marketOrder(orderId, buyOrSell, shares);

        json response = {{"status", "market_order_executed"},
                         {"orderId", orderId},
                         {"symbol", symbol}};
        res.set_content(response.dump(), "application/json");
      } catch (const std::exception &e) {
        res.status = 500;
        json response = {{"error", e.what()}};
        res.set_content(response.dump(), "application/json");
      } });

    server.Delete("/order/:symbol/:orderId", [this](const httplib::Request &req,
                                                    httplib::Response &res)
                  {
      std::string symbol(req.path_params.at("symbol"));
      int orderId = std::stoi(std::string(req.path_params.at("orderId")));

      std::lock_guard<std::mutex> lock(booksMutex);

      if (orderBooks.find(symbol) == orderBooks.end()) {
        res.status = 404;
        json response = {{"error", "Order book not found for symbol"}};
        res.set_content(response.dump(), "application/json");
        return;
      }

      orderBooks[symbol]->cancelLimitOrder(orderId);
      json response = {{"status", "order_cancelled"}, {"orderId", orderId}};
      res.set_content(response.dump(), "application/json"); });

    server.Get("/book/:symbol", [this](const httplib::Request &req,
                                       httplib::Response &res)
               {
      std::string symbol(req.path_params.at("symbol"));
      std::lock_guard<std::mutex> lock(booksMutex);

      if (orderBooks.find(symbol) == orderBooks.end()) {
        res.status = 404;
        res.set_content(json{{"error", "Order book not found"}}.dump(),
                        "application/json");
        return;
      }

      Book *book = orderBooks[symbol];

      std::vector<int> bidPrices =
          book->inOrderTreeTraversal(book->getBuyTree());
      std::reverse(bidPrices.begin(), bidPrices.end()); 
      json bids = json::array();
      for (int price : bidPrices) {
        Limit *level = book->searchLimitMaps(price, true);
        if (level)
          bids.push_back(
              {{"price", price}, {"volume", level->getTotalVolume()}});
      }

      std::vector<int> askPrices =
          book->inOrderTreeTraversal(book->getSellTree());
      json asks = json::array();
      for (int price : askPrices) {
        Limit *level = book->searchLimitMaps(price, false);
        if (level)
          asks.push_back(
              {{"price", price}, {"volume", level->getTotalVolume()}});
      }

      json response = {
          {"symbol", symbol},
          {"timestamp", std::time(nullptr)},
          {"best_bid",
           book->getHighestBuy() ? book->getHighestBuy()->getLimitPrice() : -1},
          {"best_ask",
           book->getLowestSell() ? book->getLowestSell()->getLimitPrice() : -1},
          {"bids", bids},
          {"asks", asks}};
      res.set_content(response.dump(), "application/json"); });

    server.Get("/books",
               [this](const httplib::Request &, httplib::Response &res)
               {
                 std::lock_guard<std::mutex> lock(booksMutex);

                 json books = json::array();
                 for (const auto &[symbol, book] : orderBooks)
                 {
                   books.push_back(symbol);
                 }

                 json response = {{"books", books}};
                 res.set_content(response.dump(), "application/json");
               });
  }

  void start()
  {
    std::cout << "Starting Order Book Server on " << HOST << ":" << PORT
              << std::endl;
    server.listen(HOST, PORT);
  }

private:
  bool parseJsonBody(const httplib::Request &req, json &out,
                     httplib::Response &res)
  {
    try
    {
      out = json::parse(req.body);
      return true;
    }
    catch (const json::parse_error &e)
    {
      res.status = 400;
      json response = {{"error", "Invalid JSON"}, {"details", e.what()}};
      res.set_content(response.dump(), "application/json");
      return false;
    }
  }
};

int main()
{
  OrderBookServer server;

  try
  {
    server.start();
  }
  catch (const std::exception &e)
  {
    std::cerr << "Error: " << e.what() << std::endl;
    return 1;
  }

  return 0;
}