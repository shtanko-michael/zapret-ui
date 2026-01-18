import { Download } from 'lucide-react';

interface UpdateOverlayProps {
  progress: number;
  currentVersion: string;
  newVersion: string;
}

export default function UpdateOverlay({ progress, currentVersion, newVersion }: UpdateOverlayProps) {
  return (
    <div className="fixed inset-0 bg-gradient-to-br from-blue-900 to-slate-900 flex items-center justify-center z-50">
      <div className="max-w-md w-full mx-4">
        <div className="bg-white rounded-2xl shadow-2xl p-8">
          <div className="flex justify-center mb-6">
            <div className="bg-blue-100 rounded-full p-4">
              <Download className="w-12 h-12 text-blue-600 animate-bounce" />
            </div>
          </div>

          <h2 className="text-2xl font-bold text-center text-gray-800 mb-2">
            Обновление приложения
          </h2>

          <p className="text-center text-gray-600 mb-6">
            Обновление с версии {currentVersion} на {newVersion}
          </p>

          <div className="mb-4">
            <div className="flex justify-between text-sm text-gray-700 mb-2">
              <span>Прогресс</span>
              <span className="font-medium">{progress}%</span>
            </div>
            <div className="w-full bg-gray-200 rounded-full h-3 overflow-hidden">
              <div
                className="bg-gradient-to-r from-blue-500 to-blue-600 h-full transition-all duration-300 ease-out rounded-full"
                style={{ width: `${progress}%` }}
              ></div>
            </div>
          </div>

          <p className="text-center text-sm text-gray-500 mt-6">
            Пожалуйста, подождите. Не закрывайте приложение.
          </p>
        </div>
      </div>
    </div>
  );
}
