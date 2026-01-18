import { useState, useEffect } from 'react';
import { RefreshCw, PlayCircle, Activity, AlertTriangle, AccessibilityIcon, ThumbsUp } from 'lucide-react';
import { CheckAndUpdate, GetState, RunStrategy, RunTests, StopStrategy } from '../wailsjs/go/main/App';
import type { State, Strategy, RunningInfo } from './types/models';
import StrategyCard from './components/StrategyCard';
import UpdateOverlay from './components/UpdateOverlay';

function App() {
  const [state, setState] = useState<State>();
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [isTestingAll, setIsTestingAll] = useState(false);
  const [error, setError] = useState<string>('');
  const [updateProgress, setUpdateProgress] = useState(0);
  const [isUpdating, setIsUpdating] = useState(false);

  const strategies = state?.strategies || [];
  const running: RunningInfo | undefined = state?.running || state?.config?.running;
  const currentVersion = state?.config?.version || 'n/a';
  const latestTag = state?.latestTag || '';
  const hasUpdate = state?.hasUpdate || false;
  const isTesting = isTestingAll || Boolean(state?.config?.testInProgress);

  debugger;

  const load = async () => {
    try {
      setError('');
      setIsRefreshing(true);
      const s = await GetState();
      setState(s);
    } catch (e: any) {
      setError(e?.toString() ?? 'Failed to load state');
    } finally {
      setIsRefreshing(false);
    }
  };

  useEffect(() => {
    load();
  }, []);

  const handleUpdate = async () => {
    setIsUpdating(true);
    setUpdateProgress(0);
    setError('');
    try {
      // Simulate progress
      const progressInterval = setInterval(() => {
        setUpdateProgress((prev) => {
          if (prev >= 90) {
            clearInterval(progressInterval);
            return 90;
          }
          return prev + Math.random() * 15;
        });
      }, 500);

      const s = await CheckAndUpdate();
      clearInterval(progressInterval);
      setUpdateProgress(100);
      setState(s);

      setTimeout(() => {
        setIsUpdating(false);
        setUpdateProgress(0);
      }, 1000);
    } catch (e: any) {
      setError(e?.toString() ?? 'Update failed');
      setIsUpdating(false);
      setUpdateProgress(0);
    }
  };

  const handleTests = async () => {
    setIsTestingAll(true);
    setError('');

    try {
      const s = await RunTests();
      setState(s);
      setIsTestingAll(false);
    } catch (e: any) {
      setError(e?.toString() ?? 'Tests failed');
      setIsTestingAll(false);
    }
  };

  const handleToggleStrategy = async (file: string, isRunning: boolean) => {
    setError('');
    try {
      if (isRunning) {
        const s = await StopStrategy();
        setState(s);
      } else {
        const s = await RunStrategy(file);
        setState(s);
      }
    } catch (e: any) {
      setError(e?.toString() ?? (isRunning ? 'Stop failed' : 'Run failed'));
    }
  };


  const passingTests = strategies.filter((s) => s.result?.status === 'ok').length;
  const failingTests = strategies.filter((s) => s.result?.status === 'fail').length;

  return (
    <>
      {isUpdating && (
        <UpdateOverlay
          progress={Math.round(updateProgress)}
          currentVersion={currentVersion}
          newVersion={latestTag}
        />
      )}
      <div className="min-h-screen bg-gradient-to-br from-gray-50 to-gray-200">
        <div className="max-w-7xl mx-auto px-6 py-8">
          <div className="mb-8">
            <div className="flex items-center justify-between mb-6">
              <div>
                <h1 className="text-4xl font-bold text-gray-700 mb-2 flex items-center gap-3">
                  <Activity className="w-10 h-10 text-blue-600" />
                  Zapret UI
                </h1>
                <p className='flex items-center gap-3'>
                  <div className='mx-2'>
                    Версия {currentVersion}
                  </div>
                  {latestTag && latestTag !== currentVersion && (
                    <button
                      onClick={handleUpdate}
                      className="px-6 py-3 bg-white bg-red-600 hover:bg-red-700 text-white rounded-lg font-medium shadow-md transition-all flex items-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed"
                    >
                      <AlertTriangle />
                      Обновить до {latestTag}
                    </button>
                  )}
                </p>
              </div>

              <div className="flex gap-3">
                <button
                  onClick={load}
                  disabled={isRefreshing}
                  className="px-6 py-3 bg-white hover:bg-gray-50 text-gray-700 rounded-lg font-medium shadow-md transition-all flex items-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  <RefreshCw className={`w-5 h-5 ${isRefreshing ? 'animate-spin' : ''}`} />
                  Обновить
                </button>
                <button
                  onClick={handleTests}
                  disabled={isTesting}
                  className="px-6 py-3 bg-blue-600 hover:bg-blue-700 text-white rounded-lg font-medium shadow-md transition-all flex items-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  {isTesting ? (
                    <RefreshCw className="w-5 h-5 animate-spin" />
                  ) : (
                    <PlayCircle className="w-5 h-5" />
                  )}
                  Запустить все тесты
                </button>
              </div>
            </div>

            {error && (
              <div className="mb-4 p-4 bg-red-100 border border-red-400 text-red-700 rounded-lg">
                {error}
              </div>
            )}

            <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-8">
              <div className="bg-white rounded-lg shadow-md p-6">
                <div className="flex items-center justify-between">
                  <div>
                    <p className="text-sm text-gray-600 mb-1">Запущено стратегий</p>
                    <p className="text-3xl font-bold text-gray-900">
                      {running ? 1 : 0}/{strategies.length}
                    </p>
                  </div>
                  <div className="bg-green-100 rounded-full p-3">
                    <Activity className="w-8 h-8 text-green-600" />
                  </div>
                </div>
              </div>

              <div className="bg-white rounded-lg shadow-md p-6">
                <div className="flex items-center justify-between">
                  <div>
                    <p className="text-sm text-gray-600 mb-1">Тесты пройдены</p>
                    <p className="text-3xl font-bold text-green-600">{passingTests}</p>
                  </div>
                  <div className="bg-green-100 rounded-full p-3">
                    <ThumbsUp className="w-8 h-8 text-green-600" />
                  </div>
                </div>
              </div>

              <div className="bg-white rounded-lg shadow-md p-6">
                <div className="flex items-center justify-between">
                  <div>
                    <p className="text-sm text-gray-600 mb-1">Тесты провалены</p>
                    <p className="text-3xl font-bold text-red-600">{failingTests}</p>
                  </div>
                  <div className="bg-red-100 rounded-full p-3">
                    <AccessibilityIcon className="w-8 h-8 text-red-600" />
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {strategies.map((strategy) => {
              const isRunning = Boolean(
                running?.file === strategy.name ||
                (running?.file && strategy.name.includes(running.file.replace(/\.bat$/, '')))
              );
              return (
                <StrategyCard
                  key={strategy.file}
                  strategy={strategy}
                  isRunning={isRunning}
                  runningInfo={isRunning ? running : undefined}
                  isLoading={isRefreshing}
                  onToggleStrategy={handleToggleStrategy}
                />
              );
            })}
            {strategies.length === 0 && (
              <div className="col-span-full text-center text-gray-500 py-12">
                Стратегии не найдены
              </div>
            )}
          </div>
        </div>
      </div>
    </>
  );
}

export default App;
