#include "Book.hpp"
#include "Limit.hpp"

#include <iostream>

int main() {
    Book book;

    // Seed one bid and cross it with an ask so we can sanity-check matching.
    book.addLimitOrder(1, true, 100, 100);
    book.addLimitOrder(2, false, 40, 99);

    if (book.getHighestBuy() != nullptr) {
        std::cout << "best_bid=" << book.getHighestBuy()->getLimitPrice() << "\n";
    }
    if (book.getLowestSell() != nullptr) {
        std::cout << "best_ask=" << book.getLowestSell()->getLimitPrice() << "\n";
    }

    std::cout << "executed=" << book.executedOrdersCount << "\n";
    return 0;
}
