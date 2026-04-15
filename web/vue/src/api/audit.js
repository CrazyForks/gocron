import httpClient from '../utils/httpClient'

export default {
  list (query, callback) {
    httpClient.get('/audit', query, callback)
  }
}
