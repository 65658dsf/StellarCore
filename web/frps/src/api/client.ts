import { http } from './http'
import type { ClientInfoData, KickClientResponse } from '../types/client'

export const getClients = () => {
  return http.get<ClientInfoData[]>('../api/clients')
}

export const getClient = (key: string) => {
  return http.get<ClientInfoData>(`../api/clients/${key}`)
}

export const kickClient = (runId: string) => {
  return http.post<KickClientResponse>('../api/client/kick', { runId })
}
