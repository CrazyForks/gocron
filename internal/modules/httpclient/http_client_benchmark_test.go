package httpclient

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// 测试服务器
func setupTestServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond) // 模拟处理时间
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))
}

// 测试1: 连续请求性能
func TestSequentialRequests(t *testing.T) {
	server := setupTestServer()
	defer server.Close()

	requests := 50
	start := time.Now()

	for i := 0; i < requests; i++ {
		resp := Get(server.URL, 5)
		if resp.StatusCode != 200 {
			t.Errorf("请求 %d 失败: %v", i, resp.Body)
		}
	}

	duration := time.Since(start)
	avgTime := float64(duration.Milliseconds()) / float64(requests)

	t.Logf("📊 连续请求测试 (%d 个请求):", requests)
	t.Logf("   总耗时: %v", duration)
	t.Logf("   平均耗时: %.2f ms/请求", avgTime)
}

// 测试2: 并发请求性能
func TestConcurrentRequests(t *testing.T) {
	server := setupTestServer()
	defer server.Close()

	concurrency := 20
	requestsPerWorker := 5
	totalRequests := concurrency * requestsPerWorker

	start := time.Now()
	var wg sync.WaitGroup
	wg.Add(concurrency)

	successCount := 0
	errorCount := 0
	var mu sync.Mutex

	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < requestsPerWorker; j++ {
				resp := Get(server.URL, 5)
				mu.Lock()
				if resp.StatusCode == 200 {
					successCount++
				} else {
					errorCount++
				}
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	duration := time.Since(start)
	avgTime := float64(duration.Milliseconds()) / float64(totalRequests)

	t.Logf("📊 并发请求测试 (%d 并发, %d 个请求):", concurrency, totalRequests)
	t.Logf("   总耗时: %v", duration)
	t.Logf("   平均耗时: %.2f ms/请求", avgTime)
	t.Logf("   成功: %d, 失败: %d", successCount, errorCount)
}

// 测试3: 不同超时配置
func TestDifferentTimeouts(t *testing.T) {
	server := setupTestServer()
	defer server.Close()

	timeouts := []int{5, 10, 30, 300}
	
	for _, timeout := range timeouts {
		start := time.Now()
		resp := Get(server.URL, timeout)
		duration := time.Since(start)
		
		if resp.StatusCode != 200 {
			t.Errorf("超时 %d 秒的请求失败: %v", timeout, resp.Body)
		}
		t.Logf("   超时配置 %ds: 耗时 %v", timeout, duration)
	}
}

// 基准测试1: 单个请求
func BenchmarkSingleRequest(b *testing.B) {
	server := setupTestServer()
	defer server.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Get(server.URL, 5)
	}
}

// 基准测试2: 并发请求
func BenchmarkConcurrentRequests(b *testing.B) {
	server := setupTestServer()
	defer server.Close()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			Get(server.URL, 5)
		}
	})
}

// 基准测试3: POST 请求
func BenchmarkPostRequest(b *testing.B) {
	server := setupTestServer()
	defer server.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		PostParams(server.URL, "key=value", 5)
	}
}

// 测试4: 高并发压力测试
func TestHighConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过压力测试，使用 -short 标志")
	}

	server := setupTestServer()
	defer server.Close()

	concurrency := 100
	requestsPerWorker := 10
	totalRequests := concurrency * requestsPerWorker

	t.Logf("🔥 高并发压力测试 (%d 并发, %d 个请求)", concurrency, totalRequests)

	start := time.Now()
	var wg sync.WaitGroup
	wg.Add(concurrency)

	successCount := 0
	errorCount := 0
	var mu sync.Mutex

	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < requestsPerWorker; j++ {
				resp := Get(server.URL, 5)
				mu.Lock()
				if resp.StatusCode == 200 {
					successCount++
				} else {
					errorCount++
				}
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	duration := time.Since(start)
	qps := float64(totalRequests) / duration.Seconds()

	t.Logf("📊 压力测试结果:")
	t.Logf("   总耗时: %v", duration)
	t.Logf("   QPS: %.2f", qps)
	t.Logf("   成功: %d, 失败: %d", successCount, errorCount)
	t.Logf("   成功率: %.2f%%", float64(successCount)/float64(totalRequests)*100)
}

// 测试5: 连接复用验证
func TestConnectionReuse(t *testing.T) {
	server := setupTestServer()
	defer server.Close()

	t.Log("🔍 连接复用测试 (执行 10 次请求)")
	
	for i := 0; i < 10; i++ {
		start := time.Now()
		resp := Get(server.URL, 5)
		duration := time.Since(start)
		
		if resp.StatusCode != 200 {
			t.Errorf("请求 %d 失败", i+1)
		}
		t.Logf("   请求 %d: %v", i+1, duration)
	}
	
	t.Log("💡 提示: 如果后续请求明显快于首次请求，说明连接被复用")
}
