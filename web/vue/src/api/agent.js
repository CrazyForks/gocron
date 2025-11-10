import httpClient from '../utils/httpClient'

export default {
  generateToken (callback) {
    httpClient.post('/agent/generate-token', {}, callback)
  }
}
