# user_script.py
# This is a sample user script that implements a simple trading strategy.

class Strategy:
    """
    A simple moving average crossover strategy.
    """
    def __init__(self):
        self.prices = []
        self.short_window = 5
        self.long_window = 10

    def on_market_data(self, symbol, bid, ask, last_price, volume, timestamp_ns, portfolio_value):
        """
        Called on every market data tick.
        """
        self.prices.append(last_price)
        if len(self.prices) < self.long_window:
            return "HOLD", 0, 0

        # Calculate moving averages
        short_ma = sum(self.prices[-self.short_window:]) / self.short_window
        long_ma = sum(self.prices[-self.long_window:]) / self.long_window

        # Crossover logic
        if short_ma > long_ma:
            # Buy signal
            return "BUY", 1, ask  # Buy 1 unit at the ask price
        elif short_ma < long_ma:
            # Sell signal
            return "SELL", 1, bid  # Sell 1 unit at the bid price
        else:
            # No signal
            return "HOLD", 0, 0
