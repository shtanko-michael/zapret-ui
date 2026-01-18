export interface RunningInfo {
    file: string;
    pid: number;
    startedAt: string;
}

export interface Config {
    version: string;
    lastStrategy: string;
    lastTestAt: string;
    testResults: Record<string, TestResult>;
    bestStrategy: string;
    meta?: Record<string, any>;
    running?: RunningInfo;
    testInProgress: boolean;
}

export interface TestResult {
    name: string;
    httpOk: number;
    httpErr: number;
    httpUnsup: number;
    pingOk: number;
    pingFail: number;
    fail: number;
    blocked: number;
    status: string;
    lastTestedAt?: string;
}

export interface Strategy {
    name: string;
    file: string;
    result?: TestResult;
    best?: boolean;
}

export interface State {
    config?: Config;
    strategies?: Strategy[];
    latestTag?: string;
    hasUpdate?: boolean;
    currentPath?: string;
    lastTestLog?: string;
    running?: RunningInfo;
}
