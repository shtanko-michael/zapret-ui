import { Server, Play, Square, CheckCircle, XCircle, Clock, Crown, CrownIcon } from 'lucide-react';
import type { Strategy, RunningInfo } from '../types/models';

interface StrategyCardProps {
  strategy: Strategy;
  isRunning: boolean;
  runningInfo?: RunningInfo;
  isLoading?: boolean;
  onToggleStrategy: (file: string, isRunning: boolean) => void;
}

export default function StrategyCard({ 
  strategy, 
  isRunning, 
  runningInfo,
  isLoading, 
  onToggleStrategy 
}: StrategyCardProps) {
  if (isLoading) {
    return (
      <div className="bg-white rounded-lg shadow-md p-6 animate-pulse">
        <div className="h-6 bg-gray-200 rounded mb-4 w-3/4"></div>
        <div className="h-4 bg-gray-200 rounded mb-2 w-1/2"></div>
        <div className="h-4 bg-gray-200 rounded mb-4 w-2/3"></div>
        <div className="h-10 bg-gray-200 rounded"></div>
      </div>
    );
  }

  const getTestStatusIcon = () => {
    if (!strategy.result) {
      return <Clock className="w-5 h-5 text-gray-400" />;
    }
    switch (strategy.result.status) {
      case 'ok':
        return <CheckCircle className="w-5 h-5 text-green-600" />;
      case 'fail':
        return <XCircle className="w-5 h-5 text-red-600" />;
      default:
        return <Clock className="w-5 h-5 text-gray-400" />;
    }
  };

  const getTestStatusText = () => {
    if (!strategy.result) {
      return 'Тесты не запущены';
    }
    switch (strategy.result.status) {
      case 'ok':
        return 'Тесты пройдены';
      case 'fail':
        return 'Тесты провалены';
      default:
        return 'Тесты не запущены';
    }
  };

  const getTestSummary = () => {
    if (!strategy.result) {
      return 'Нет данных';
    }
    const parts = [];
    if (strategy.result.httpOk > 0) parts.push(`OK:${strategy.result.httpOk}`);
    if (strategy.result.httpErr > 0) parts.push(`ERR:${strategy.result.httpErr}`);
    if (strategy.result.fail > 0) parts.push(`FAIL:${strategy.result.fail}`);
    if (strategy.result.pingFail > 0) parts.push(`PING_FAIL:${strategy.result.pingFail}`);
    return parts.length > 0 ? parts.join(' ') : 'Нет данных';
  };

  return (
    <div className="bg-white rounded-lg shadow-md p-6 hover:shadow-lg transition-shadow">
      <div className="flex items-start justify-between mb-4">
        <div className="flex items-center gap-3">
          <Server className="w-6 h-6 text-blue-600" />
          <h3 className="text-lg font-semibold text-gray-800">{strategy.name}</h3>
        </div>
        <div className="flex items-center gap-2">
          {strategy.best && (
            <span className="px-3 py-3 bg-yellow-100 text-yellow-800 rounded-full text-xs font-medium flex items-center gap-3">
              <CrownIcon className='' />
              <span className='font-bold'>BEST</span>
            </span>
          )}
          <div
            className={`px-3 py-3 rounded-full text-sm font-medium ${
              isRunning
                ? 'bg-green-100 text-green-800'
                : 'bg-gray-100 text-gray-800'
            }`}
          >
            {isRunning ? 'Запущен' : 'Остановлен'}
          </div>
        </div>
      </div>

      <div className="space-y-3 mb-4">
        <div className="flex items-center gap-2 text-sm">
          {getTestStatusIcon()}
          <span className="text-gray-700">{getTestStatusText()}</span>
        </div>
        <div className="text-sm text-gray-600">
          {getTestSummary()}
        </div>
        {isRunning && runningInfo && (
          <div className="text-sm text-gray-600">
            PID: <span className="font-medium">{runningInfo.pid}</span>
          </div>
        )}
      </div>

      <button
        onClick={() => onToggleStrategy(strategy.file, isRunning)}
        className={`w-full py-2.5 px-4 rounded-lg font-medium transition-colors flex items-center justify-center gap-2 ${
          isRunning
            ? 'bg-red-600 hover:bg-red-700 text-white'
            : 'bg-blue-600 hover:bg-blue-700 text-white'
        }`}
      >
        {isRunning ? (
          <>
            <Square className="w-4 h-4" />
            Остановить
          </>
        ) : (
          <>
            <Play className="w-4 h-4" />
            Запустить
          </>
        )}
      </button>
    </div>
  );
}
