import { http } from './http'
import type { ServerInfo } from '../types/server'
import type { TrafficResponse } from '../types/proxy'

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
