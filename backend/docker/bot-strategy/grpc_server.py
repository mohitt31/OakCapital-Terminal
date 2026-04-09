# grpc_server.py
import grpc
from concurrent import futures
import time
import importlib.util
import sys

# Import the generated classes
import strategy_pb2
import strategy_pb2_grpc

# Path to the user's script, which will be mounted into the container.
# We expect the user's script to be named 'user_script.py'.
USER_SCRIPT_PATH = "user_script.py"

class TradingStrategyServicer(strategy_pb2_grpc.TradingStrategyServicer):
    def __init__(self, user_strategy):
        self.user_strategy = user_strategy

    def OnMarketData(self, request, context):
        """
        Receives market data from the Go application, passes it to the user's
        Python script, and returns the script's decision.
        """
        try:
            # The user's script is expected to have an 'on_market_data' method.
            action, quantity, limit_price = self.user_strategy.on_market_data(
                symbol=request.symbol,
                bid=request.bid,
                ask=request.ask,
                last_price=request.last_price,
                volume=request.volume,
                timestamp_ns=request.timestamp_ns,
                portfolio_value=request.portfolio_value
            )

            # Convert the action string to the enum value.
            action_enum = strategy_pb2.TradeDecision.Action.Value(action.upper())

            return strategy_pb2.TradeDecision(
                action=action_enum,
                quantity=quantity,
                limit_price=limit_price
            )
        except Exception as e:
            print(f"Error in user script: {e}")
            context.abort(grpc.StatusCode.INTERNAL, f"Error in user script: {e}")
            return strategy_pb2.TradeDecision()

def load_user_strategy():
    """
    Dynamically loads the user's strategy from the 'user_script.py' file.
    The script is expected to have a class named 'Strategy'.
    """
    spec = importlib.util.spec_from_file_location("user_script", USER_SCRIPT_PATH)
    user_script_module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(user_script_module)
    
    # The user's script must have a class named 'Strategy'.
    if not hasattr(user_script_module, 'Strategy'):
        raise ImportError("User script must contain a class named 'Strategy'")
        
    return user_script_module.Strategy()

def serve():
    """
    Starts the gRPC server.
    """
    try:
        user_strategy = load_user_strategy()
    except Exception as e:
        print(f"Failed to load user strategy: {e}")
        sys.exit(1)

    server = grpc.server(futures.ThreadPoolExecutor(max_workers=1))
    strategy_pb2_grpc.add_TradingStrategyServicer_to_server(
        TradingStrategyServicer(user_strategy), server
    )
    server.add_insecure_port('[::]:50051')
    print("gRPC server started on port 50051")
    server.start()
    
    # Keep the server running.
    try:
        while True:
            time.sleep(86400)
    except KeyboardInterrupt:
        server.stop(0)

if __name__ == '__main__':
    serve()
