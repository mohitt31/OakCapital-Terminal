import { useNavigate } from 'react-router-dom';
import { AlignJustify, TrendingUp, Clock } from 'lucide-react';

const DesktopPage = () => {
  const navigate = useNavigate();

  return (
    <div className="min-h-screen bg-[#0B0E11] text-white selection:bg-blue-500/30 overflow-x-hidden">
      {/* Navigation - PRESERVED FROM PREVIOUS */}
      <nav className="flex items-center justify-between px-6 py-4 md:px-12 backdrop-blur-md sticky top-0 z-50 border-b border-white/5 bg-[#0d1017]/80">
        <div className="flex items-center gap-2">
          {/* Logo Placeholder */}
          <div className="w-8 h-8 rounded bg-gradient-to-br from-blue-600 to-purple-600 flex items-center justify-center text-white font-bold text-xl">
            N
          </div>
          <span className="font-bold text-lg tracking-tight">NEXTBULL</span>
        </div>
        
        <div className="hidden md:flex items-center gap-8 text-sm text-gray-400 font-medium">
          <a href="#" className="hover:text-white transition-colors">About</a>
          <a href="#" className="hover:text-white transition-colors">Products</a>
          <button onClick={() => navigate('/markets')} className="hover:text-white transition-colors">Markets</button>
          <button onClick={() => navigate('/terminal')} className="hover:text-white transition-colors">Terminal</button>
          <a href="#" className="hover:text-white transition-colors flex items-center gap-1">
            More <span className="text-xs">▼</span>
          </a>
        </div>

        <div className="flex items-center gap-6 text-sm font-medium">
          <button onClick={() => navigate('/signin')} className="hover:text-gray-300 transition-colors">Sign In</button>
          <button onClick={() => navigate('/login')} className="bg-white text-black px-4 py-2 rounded-md hover:bg-gray-200 transition-colors font-semibold">Sign up</button>
        </div>
      </nav>

      {/* Ticker Tape */}
      <div className="w-full bg-[#10141A] border-b border-[#2B2F36] py-2.5 flex items-center overflow-hidden text-xs font-mono">
        <div className="flex w-[200%] animate-marquee">
          {/* Duplicate set for seamless scrolling */}
          <div className="flex w-full justify-around pr-4">
              <button className="flex gap-2" onClick={() => navigate('/terminal?symbol=RELIANCE')}>
                  <span className="text-gray-300 font-bold">RELIANCE</span>
                  <span className="text-[#00C076]">2964.20</span>
                  <span className="text-[#00C076] bg-[#00C076]/10 px-1 rounded">+1.41%</span>
              </button>
              <button className="flex gap-2" onClick={() => navigate('/terminal?symbol=TCS')}>
                  <span className="text-gray-300 font-bold">TCS</span>
                  <span className="text-[#FF3B30]">4178.50</span>
                  <span className="text-[#FF3B30] bg-[#FF3B30]/10 px-1 rounded">-0.77%</span>
              </button>
              <button className="flex gap-2" onClick={() => navigate('/terminal?symbol=HDFCBANK')}>
                  <span className="text-gray-300 font-bold">HDFCBANK</span>
                  <span className="text-[#00C076]">1652.10</span>
                  <span className="text-[#00C076] bg-[#00C076]/10 px-1 rounded">+0.51%</span>
              </button>
              <button className="flex gap-2" onClick={() => navigate('/terminal?symbol=ICICIBANK')}>
                  <span className="text-gray-300 font-bold">ICICIBANK</span>
                  <span className="text-[#00C076]">1198.40</span>
                  <span className="text-[#00C076] bg-[#00C076]/10 px-1 rounded">+2.34%</span>
              </button>
              <button className="flex gap-2" onClick={() => navigate('/terminal?symbol=SBIN')}>
                  <span className="text-gray-300 font-bold">SBIN</span>
                  <span className="text-[#FF3B30]">812.90</span>
                  <span className="text-[#FF3B30] bg-[#FF3B30]/10 px-1 rounded">-0.42%</span>
              </button>
          </div>
          <div className="flex w-full justify-around pr-4">
              <button className="flex gap-2" onClick={() => navigate('/terminal?symbol=RELIANCE')}>
                  <span className="text-gray-300 font-bold">RELIANCE</span>
                  <span className="text-[#00C076]">2964.20</span>
                  <span className="text-[#00C076] bg-[#00C076]/10 px-1 rounded">+1.41%</span>
              </button>
              <button className="flex gap-2" onClick={() => navigate('/terminal?symbol=TCS')}>
                  <span className="text-gray-300 font-bold">TCS</span>
                  <span className="text-[#FF3B30]">4178.50</span>
                  <span className="text-[#FF3B30] bg-[#FF3B30]/10 px-1 rounded">-0.77%</span>
              </button>
              <button className="flex gap-2" onClick={() => navigate('/terminal?symbol=HDFCBANK')}>
                  <span className="text-gray-300 font-bold">HDFCBANK</span>
                  <span className="text-[#00C076]">1652.10</span>
                  <span className="text-[#00C076] bg-[#00C076]/10 px-1 rounded">+0.51%</span>
              </button>
              <button className="flex gap-2" onClick={() => navigate('/terminal?symbol=ICICIBANK')}>
                  <span className="text-gray-300 font-bold">ICICIBANK</span>
                  <span className="text-[#00C076]">1198.40</span>
                  <span className="text-[#00C076] bg-[#00C076]/10 px-1 rounded">+2.34%</span>
              </button>
              <button className="flex gap-2" onClick={() => navigate('/terminal?symbol=SBIN')}>
                  <span className="text-gray-300 font-bold">SBIN</span>
                  <span className="text-[#FF3B30]">812.90</span>
                  <span className="text-[#FF3B30] bg-[#FF3B30]/10 px-1 rounded">-0.42%</span>
              </button>
          </div>
        </div>
      </div>

      <main className="flex flex-col items-center w-full px-6 md:px-12 max-w-[1400px] mx-auto">
        {/* HERO SECTION */}
        <section className="w-full pt-20 pb-16 grid grid-cols-1 lg:grid-cols-[1fr_1.2fr] gap-12 lg:gap-20 items-center">
            {/* Left Content */}
            <div className="flex flex-col items-start text-left space-y-6">
                <div className="inline-flex items-center gap-2 bg-blue-900/40 border border-blue-500/30 text-blue-300 text-xs font-semibold px-4 py-1.5 rounded-full shadow-[0_0_15px_rgba(59,130,246,0.2)]">
                    <span className="w-2 h-2 rounded-full bg-blue-400 animate-pulse"></span>
                    Live synthetic market - 87 msgs/sec
                </div>

                <h1 className="text-5xl md:text-[64px] font-extrabold leading-[1.1] tracking-tight text-white m-0 py-2">
                    Trade smarter<br />
                    with<br />
                    <span className="text-blue-500">AI-powered bots</span>
                </h1>

                <p className="text-gray-400 text-lg leading-relaxed max-w-lg mt-2 font-medium">
                    A fully simulated crypto exchange with real-time order books, GBM-driven price generation, and algorithmic trading bots — all running locally with zero external data dependencies.
                </p>

                <div className="flex flex-wrap gap-4 pt-4">
                    <button onClick={() => navigate('/login')} className="px-8 py-3.5 rounded-lg border border-white/10 hover:border-white/20 hover:bg-white/5 transition-all text-sm font-semibold text-gray-300">
                        Start trading free
                    </button>
                    <button onClick={() => navigate('/terminal')} className="px-8 py-3.5 rounded-lg bg-blue-600 hover:bg-blue-500 transition-all text-sm font-semibold text-white">
                        Open terminal
                    </button>
                    <button className="px-8 py-3.5 rounded-lg border border-transparent bg-[#141824] hover:bg-[#1c2230] transition-all text-sm font-semibold text-gray-300" onClick={() => navigate('/signin')}>
                        Sign in to account
                    </button>
                </div>
            </div>

            {/* Right Content - Mock Chart Terminal */}
            <div className="w-full h-full min-h-[450px] bg-[#141821] border border-blue-900/30 rounded-2xl relative shadow-[0_0_40px_rgba(30,58,138,0.15)] p-6 flex flex-col pt-5 pl-6 pb-6 pr-6">
                <div className="flex justify-between items-start mb-6 w-full">
                    <div>
                        <div className="text-gray-200 font-bold text-lg">BTC / USD</div>
                        <div className="text-gray-500 text-xs font-medium mt-0.5">Synthetic - GBM model</div>
                    </div>
                    <div className="text-right">
                        <div className="text-green-500 font-bold text-2xl tracking-tight">42448.77</div>
                        <div className="text-green-500 text-sm font-bold bg-green-500/10 inline-block px-1.5 rounded mt-1">+2.34%</div>
                    </div>
                </div>
                
                {/* Mock Chart Area */}
                <div className="flex-1 min-h-[160px] border-b border-white/5 relative flex items-end justify-between px-2 pb-8 mb-4">
                   {/* Fake candles mimicking the snapshot closely */}
                   {[12, 18, 10, 24, 15, 8, -10, -20, -15, -8, 5, 20, 25, 18, 12, -5, -15, 10, 22, 16, 5, -8, -12, -22].map((val, i) => {
                       const isUp = val > 0;
                       const height = Math.abs(val) * 2 + 10;
                       const wickHeight = Math.floor(Math.random() * 8) * 4 + 10;
                       return (
                           <div key={i} className="flex flex-col items-center justify-center w-1.5 h-full relative group">
                               <div style={{ height: `${wickHeight}px` }} className={`absolute ${isUp ? 'bottom-[50%]' : 'top-[50%]'} w-[1px] ${isUp ? 'bg-green-500' : 'bg-red-500'} opacity-60`}></div>
                               <div 
                                style={{ height: `${height}px`, margin: isUp ? `0 0 ${val}px 0` : `${Math.abs(val)}px 0 0 0` }}
                                className={`w-2.5 z-10 ${isUp ? 'bg-[#00c853]' : 'bg-[#d50000]'} rounded-sm border-[0.5px] border-black/20 group-hover:brightness-125 transition-all`}></div>
                           </div>
                       );
                   })}
                </div>

                {/* Mock Order Book */}
                <div className="grid grid-cols-2 gap-8 text-[11px] font-mono w-full">
                    <div className="flex flex-col gap-1">
                        <div className="text-gray-500 mb-1 font-semibold flex justify-between">
                            <span>ASKS</span>
                            <span></span>
                        </div>
                        <div className="flex justify-between hover:bg-white/5 px-1 rounded transition-colors"><span className="text-[#ff5252]">42445.51</span><span className="text-gray-400">1.921</span></div>
                        <div className="flex justify-between hover:bg-white/5 px-1 rounded transition-colors"><span className="text-[#ff5252]">42445.02</span><span className="text-gray-400">1.805</span></div>
                        <div className="flex justify-between hover:bg-white/5 px-1 rounded transition-colors"><span className="text-[#ff5252]">42445.92</span><span className="text-gray-400">0.218</span></div>
                        <div className="flex justify-between hover:bg-white/5 px-1 rounded transition-colors"><span className="text-[#ff5252]">42446.26</span><span className="text-gray-400">0.877</span></div>
                        <div className="flex justify-between hover:bg-white/5 px-1 rounded transition-colors"><span className="text-[#ff5252]">42446.51</span><span className="text-gray-400">1.140</span></div>
                    </div>
                    <div className="flex flex-col gap-1">
                        <div className="text-gray-500 mb-1 font-semibold flex justify-between">
                            <span></span>
                            <span>BIDS</span>
                        </div>
                        <div className="flex justify-between hover:bg-white/5 px-1 rounded transition-colors"><span className="text-gray-400">42444.86</span><span className="text-[#00e676]">2.776</span></div>
                        <div className="flex justify-between hover:bg-white/5 px-1 rounded transition-colors"><span className="text-gray-400">42444.32</span><span className="text-[#00e676]">0.193</span></div>
                        <div className="flex justify-between hover:bg-white/5 px-1 rounded transition-colors"><span className="text-gray-400">42444.08</span><span className="text-[#00e676]">0.844</span></div>
                        <div className="flex justify-between hover:bg-white/5 px-1 rounded transition-colors"><span className="text-gray-400">42443.82</span><span className="text-[#00e676]">2.427</span></div>
                        <div className="flex justify-between hover:bg-white/5 px-1 rounded transition-colors"><span className="text-gray-400">42443.65</span><span className="text-[#00e676]">2.648</span></div>
                    </div>
                </div>
            </div>
        </section>

        {/* STATS SECTION */}
        <section className="w-full py-16 flex flex-wrap gap-12 sm:gap-16 lg:gap-24">
            <div className="flex flex-col">
                <div className="text-3xl md:text-4xl font-extrabold text-white mb-2 tracking-tight">$100k</div>
                <div className="text-[11px] text-gray-500 uppercase tracking-[0.2em] font-semibold leading-relaxed">Starting<br/>capital</div>
            </div>
            <div className="flex flex-col">
                <div className="text-3xl md:text-4xl font-extrabold text-[#00e676] mb-2 tracking-tight">6 bots</div>
                <div className="text-[11px] text-blue-400 uppercase tracking-[0.2em] font-semibold leading-relaxed">Competing<br/>live</div>
            </div>
            <div className="flex flex-col">
                <div className="text-3xl md:text-4xl font-extrabold text-white mb-2 tracking-tight">50–100</div>
                <div className="text-[11px] text-gray-500 uppercase tracking-[0.2em] font-semibold leading-relaxed">Messages/sec</div>
            </div>
            <div className="flex flex-col">
                <div className="text-3xl md:text-4xl font-extrabold text-[#00e676] mb-2 tracking-tight">0</div>
                <div className="text-[11px] text-gray-500 uppercase tracking-[0.2em] font-semibold leading-relaxed">External<br/>APIs</div>
            </div>
        </section>

        {/* FEATURES CARDS SECTION */}
        <section className="w-full py-16 mt-8">
            <div className="text-[11px] font-bold text-gray-400/80 uppercase tracking-[0.2em] mb-8 flex items-center gap-4">
                BUILT FOR IIT KHARAGPUR · OPEN SOFT 2026
            </div>
            
            <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
                {/* Card 1 */}
                <div className="bg-[#11141c] border border-white/5 rounded-2xl p-7 hover:border-blue-500/30 transition-all hover:-translate-y-1 hover:shadow-[0_10px_30px_-15px_rgba(59,130,246,0.3)] group">
                    <div className="w-12 h-12 rounded-xl bg-[#1a2333] flex items-center justify-center text-blue-400 mb-6 group-hover:scale-110 transition-transform">
                        <AlignJustify className="w-5 h-5" />
                    </div>
                    <h3 className="text-lg font-bold text-gray-200 mb-3">Limit Order Book</h3>
                    <p className="text-[13px] text-gray-400 leading-relaxed font-medium">
                        In-memory LOB with Price-Time priority. Supports limit, market, and cancel order types with microsecond matching.
                    </p>
                </div>

                {/* Card 2 */}
                <div className="bg-[#11141c] border border-white/5 rounded-2xl p-7 hover:border-green-500/30 transition-all hover:-translate-y-1 hover:shadow-[0_10px_30px_-15px_rgba(34,197,94,0.3)] group">
                    <div className="w-12 h-12 rounded-xl bg-[#162a22] flex items-center justify-center text-green-400 mb-6 group-hover:scale-110 transition-transform">
                        <TrendingUp className="w-5 h-5" />
                    </div>
                    <h3 className="text-lg font-bold text-gray-200 mb-3">GBM Price Engine</h3>
                    <p className="text-[13px] text-gray-400 leading-relaxed font-medium">
                        Geometric Brownian Motion drives synthetic price generation. Configurable μ drift and σ volatility parameters.
                    </p>
                </div>

                {/* Card 3 */}
                <div className="bg-[#11141c] border border-white/5 rounded-2xl p-7 hover:border-orange-500/30 transition-all hover:-translate-y-1 hover:shadow-[0_10px_30px_-15px_rgba(249,115,22,0.3)] group">
                    <div className="w-12 h-12 rounded-xl bg-[#2a2216] flex items-center justify-center text-orange-400 mb-6 group-hover:scale-110 transition-transform">
                        <Clock className="w-5 h-5" />
                    </div>
                    <h3 className="text-lg font-bold text-gray-200 mb-3">Real-time WebSocket</h3>
                    <p className="text-[13px] text-gray-400 leading-relaxed font-medium">
                        50-100 order messages per second streamed to frontend. React renders candlestick charts, depth ladder, and trade feed live.
                    </p>
                </div>
            </div>
        </section>

        {/* Footer / Call to action - PRESERVED FROM PREVIOUS */}
        <section className="w-full flex justify-center pb-24 px-6 relative mt-16 border-t border-white/5 pt-20">
             <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-full max-w-3xl h-64 bg-white/5 blur-[120px] rounded-full -z-10"></div>
          <div className="text-center">
            <h2 className="text-4xl md:text-5xl font-bold mb-4">
              Built by Fintech Engineers,<br/>for <span className="text-purple-500">Your Dominance</span>
            </h2>
            <div className="mt-12 rounded-xl overflow-hidden border border-white/10 p-2 bg-white/5 max-w-4xl mx-auto shadow-2xl">
              {/* Setup image mockup */}
              <div className="aspect-video bg-[#1a1a1a] rounded-lg border border-white/5 relative flex items-center justify-center overflow-hidden">
                <div className="text-gray-500 text-xl font-medium">Trading Setup Image Placeholder</div>
                 {/* Fake screens glow */}
                 <div className="absolute inset-0 bg-blue-500/10 blur-[80px]"></div>
              </div>
            </div>

            <div className="mt-24 space-y-6">
                <h2 className="text-4xl md:text-5xl font-extrabold tracking-tight">
                    <span className="text-blue-500">Dominate</span> every trade<span className="text-blue-500">.</span>
                </h2>
                <p className="text-gray-400 font-medium">
                    Harness the power of most powerful financial platform in the world.
                </p>
                <button 
                  onClick={() => navigate('/login')}
                  className="bg-blue-600 hover:bg-blue-500 text-white font-bold py-3 px-8 rounded-full transition-colors mt-4"
                >
                    Sign up
                </button>
            </div>
          </div>
        </section>
      </main>
    </div>
  );
};

export default DesktopPage;
