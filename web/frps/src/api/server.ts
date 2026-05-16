import { http } from './http'
import type { ServerInfo } from '../types/server'
import type { TrafficResponse } from '../types/proxy'

export type LogEntry = {
  id: number
  time: string
  level: string
  message: string
}

export type LogsResponse = {
  entries: LogEntry[]
  nextCursor: number
  truncated: boolean
  limit: number
}

export type UpdateResponse = {
  currentVersion: string
  latestVersion?: string
  hasUpdate: boolean
  updateStarted: boolean
  message: string
}

export const getServerInfo = () => {
  return http.get<ServerInfo>('../api/serverinfo')
}

export const getAllTraffic = () => {
  return http.get<TrafficResponse>('../api/traffic')
}

export const getConfigContent = () => {
  return http.getText('../api/config')
}

export const restartService = () => {
  return http.post<void>('../api/restart')
}

export const getLogs = (params: {
  cursor?: number
  limit?: number
  level?: string
}) => {
  const query = new URLSearchParams()
  if (params.cursor) {
    query.set('cursor', String(params.cursor))
  }
  if (params.limit) {
    query.set('limit', String(params.limit))
  }
  if (params.level) {
    query.set('level', params.level)
  }
  const suffix = query.toString()
  return http.get<LogsResponse>(`../api/logs${suffix ? `?${suffix}` : ''}`)
}

export const checkUpdateAndInstall = () => {
  return http.post<UpdateResponse>('../api/update')
}
