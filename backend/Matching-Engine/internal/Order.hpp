#ifndef ORDER_HPP
#define ORDER_HPP

class Limit;

class Order {
private:
    int idNumber;
    int shares;
    Order *nextOrder;
    Order *prevOrder;
    Limit *parentLimit;

    friend class Limit;
public:
    bool buyOrSell;
    int limit;

    Order(int _idNumber, bool _buyOrSell, int _shares, int _limit);

    int getShares() const;
    int getOrderId() const;
    bool getBuyOrSell() const;
    int getLimit() const;
    Limit* getParentLimit() const;

    void partiallyFillOrder(int orderedShares);
    void cancel();
    void execute();
    void modifyOrder(int newShares, int newLimit);
    void setShares(int newShares);

    void print() const;
};

#endif