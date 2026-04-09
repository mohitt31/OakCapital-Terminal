import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Mail, Lock, ArrowRight } from 'lucide-react';

const SignInPage = () => {
  const navigate = useNavigate();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [isLoading, setIsLoading] = useState(false);

  const handleSignIn = (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);
    // Simulate sign in delay
    setTimeout(() => {
      setIsLoading(false);
      navigate('/terminal');
    }, 1000);
  };

  return (
    <div className="min-h-screen bg-[#0d1017] text-white flex font-sans overflow-hidden">
      {/* Left Column - Visuals & Branding */}
      <div className="hidden lg:flex flex-col w-1/2 relative p-12 lg:p-20 justify-between bg-gradient-to-br from-[#0a0d14] to-[#11141c] border-r border-white/5">
        <div className="relative z-10">
          <button 
            onClick={() => navigate('/')}
            className="flex items-center gap-2 hover:opacity-80 transition-opacity mb-16"
          >
            <div className="w-8 h-8 rounded bg-gradient-to-br from-blue-600 to-purple-600 flex items-center justify-center text-white font-bold text-xl">
              N
            </div>
            <span className="font-bold text-xl tracking-tight">NEXTBULL</span>
          </button>

          <h1 className="text-4xl xl:text-5xl font-extrabold tracking-tight mb-4 text-transparent bg-clip-text bg-gradient-to-r from-blue-400 to-purple-500">
            Welcome back
          </h1>
          <p className="text-gray-400 text-lg mb-8 max-w-md">
            Sign in to access your dashboard, trade on the simulated exchange, and monitor your AI bots.
          </p>
        </div>

        {/* Abstract Floating Elements (Mocking the 3D graphics) */}
        <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-full h-full pointer-events-none">
            {/* Glow background */}
            <div className="absolute inset-0 bg-[radial-gradient(circle_at_center,theme(colors.blue.900/10),transparent_50%)]"></div>
            
            {/* Mock abstract blobs */}
            <div className="absolute top-[30%] left-[25%] w-32 h-32 bg-gradient-to-br from-blue-500 to-indigo-500 rounded-full blur-[2px] opacity-80 animate-pulse shadow-[0_0_50px_rgba(59,130,246,0.4)]"></div>
            <div className="absolute bottom-[25%] right-[30%] w-40 h-40 bg-gradient-to-tr from-purple-600 to-pink-400 rounded-3xl rotate-12 blur-[1px] opacity-90 animate-bounce shadow-[0_0_60px_rgba(168,85,247,0.5)]" style={{ animationDuration: '5s' }}></div>
            
            {/* Stars/Dots overlay */}
            <div className="absolute inset-0" style={{ backgroundImage: 'radial-gradient(rgba(255,255,255,0.1) 1px, transparent 1px)', backgroundSize: '40px 40px' }}></div>
        </div>
      </div>

      {/* Right Column - Form */}
      <div className="w-full lg:w-1/2 flex flex-col items-center p-8 sm:p-12 xl:p-24 overflow-y-auto h-screen">
        <div className="w-full max-w-[420px] my-auto py-8">
          
          <h2 className="text-2xl font-semibold mb-8">Sign in to your account</h2>

          {/* Social Logins */}
          <div className="space-y-3 mb-8">
            <button className="w-full py-2.5 px-4 bg-[#1a1d24] hover:bg-[#232730] border border-white/10 rounded-lg text-sm font-medium transition-colors flex items-center justify-center gap-3">
              <svg className="w-4 h-4" viewBox="0 0 24 24"><path fill="currentColor" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.59c2.1-1.92 3.31-4.74 3.31-8.09z" /><path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.59-2.77c-.98.66-2.23 1.06-3.69 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" /><path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z" /><path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" /></svg>
              Sign in with Google
            </button>
            <button className="w-full py-2.5 px-4 bg-[#1a1d24] hover:bg-[#232730] border border-white/10 rounded-lg text-sm font-medium transition-colors flex items-center justify-center gap-3">
               <svg className="w-4 h-4 text-white" viewBox="0 0 24 24" fill="currentColor"><path d="M17.05 20.28c-.98.95-2.05.8-3.08.35-1.09-.46-2.09-.48-3.24 0-1.44.62-2.2.44-3.06-.35C2.79 15.25 3.51 7.59 9.05 7.31c1.35.07 2.29.74 3.08.8 1.18-.04 2.26-.7 3.59-.76 1.34-.04 2.45.38 3.28 1.07-2.81 1.68-2.31 5.37.5 6.44-1.02 2.67-1.39 4.41-2.45 5.42zm-3.32-15.1c-.08 1.58-1.29 2.94-2.88 2.98-1.57.06-2.92-1.35-2.94-2.86.04-1.48 1.14-2.88 2.67-2.96 1.57-.08 2.92 1.33 3.15 2.84z"/></svg>
               Sign in with Apple
            </button>
          </div>

          <div className="relative flex items-center justify-center mb-8">
            <div className="absolute border-t border-white/10 w-full"></div>
            <span className="bg-[#0d1017] px-4 text-xs text-gray-500 relative">or</span>
          </div>

          {/* Form */}
          <form onSubmit={handleSignIn} className="space-y-5">
            
            {/* Email */}
            <div className="space-y-1.5">
              <label className="text-[13px] font-medium text-gray-200">Email address</label>
              <div className="relative">
                 <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                  <Mail className="h-4 w-4 text-gray-400" />
                </div>
                <input
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  className="w-full bg-[#11141c] border border-white/10 rounded-md py-2 pl-9 pr-3 text-sm text-white placeholder-gray-500 focus:outline-none focus:ring-1 focus:ring-blue-500 focus:border-blue-500 transition-colors"
                  placeholder="name@company.com"
                  required
                />
              </div>
            </div>

            {/* Password */}
            <div className="space-y-1.5 pt-2">
              <div className="flex items-center justify-between">
                <label className="text-[13px] font-medium text-gray-200">Password</label>
                <a href="#" className="text-[12px] text-blue-400 hover:text-blue-300 transition-colors">Forgot password?</a>
              </div>
              <div className="relative">
                 <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                  <Lock className="h-4 w-4 text-gray-400" />
                </div>
                <input
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  className="w-full bg-[#11141c] border border-white/10 rounded-md py-2 pl-9 pr-3 text-sm text-white placeholder-gray-500 focus:outline-none focus:ring-1 focus:ring-blue-500 focus:border-blue-500 transition-colors"
                  placeholder="••••••••"
                  required
                />
              </div>
            </div>

            {/* Submit Button */}
            <div className="pt-6">
              <button
                type="submit"
                disabled={isLoading}
                className="w-full bg-blue-600 hover:bg-blue-500 text-white font-semibold py-2.5 rounded-md transition-colors flex items-center justify-center gap-2"
              >
                {isLoading ? 'Signing in...' : (
                    <>
                    Sign in <ArrowRight className="h-4 w-4" />
                    </>
                )}
              </button>
            </div>

          </form>

          {/* Toggle Link */}
          <div className="mt-8 text-center text-[13px] text-gray-400">
            Don't have an account?{' '}
            <button 
                onClick={() => navigate('/login')}
                className="text-white font-semibold hover:text-blue-400 transition-colors"
            >
                Sign up
            </button>
          </div>
        </div>
      </div>
    </div>
  );
};

export default SignInPage;
