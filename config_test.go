package octo

import (
    "testing"
    "time"
)

func TestHeaderSizeSettings(t *testing.T) {
    // Test default value
    if GetMaxHeaderSize() != 1024*1024 {
        t.Errorf("Default header size should be 1MB, got %d", GetMaxHeaderSize())
    }

    // Test changing the value
    newSize := int64(2 * 1024 * 1024) // 2MB
    ChangeMaxHeaderSize(newSize)
    if GetMaxHeaderSize() != newSize {
        t.Errorf("Header size should be %d after change, got %d", newSize, GetMaxHeaderSize())
    }

    // Test that NewHTTPServer uses the configured value
    server := NewHTTPServer(":8080", nil)
    if server.MaxHeaderBytes != int(newSize) {
        t.Errorf("Server MaxHeaderBytes should be %d, got %d", newSize, server.MaxHeaderBytes)
    }

    // Test NewHTTPServerWithConfig
    customServer := NewHTTPServerWithConfig(":8080", nil, 5*time.Second, 5*time.Second, 10*time.Second)
    if customServer.MaxHeaderBytes != int(newSize) {
        t.Errorf("Custom server MaxHeaderBytes should be %d, got %d", newSize, customServer.MaxHeaderBytes)
    }
    if customServer.ReadTimeout != 5*time.Second {
        t.Errorf("Custom server ReadTimeout should be 5s, got %v", customServer.ReadTimeout)
    }

    // Reset to default for other tests
    ChangeMaxHeaderSize(1024 * 1024)
}

func TestSetupOctoWithHeaderSize(t *testing.T) {
    logger := GetLogger() // Save current logger
    
    // Test SetupOcto with header size
    newHeaderSize := int64(5 * 1024 * 1024) // 5MB
    newBodySize := int64(20 * 1024 * 1024)  // 20MB
    
    SetupOcto(nil, newBodySize, newHeaderSize)
    
    if GetMaxBodySize() != newBodySize {
        t.Errorf("Body size should be %d after SetupOcto, got %d", newBodySize, GetMaxBodySize())
    }
    
    if GetMaxHeaderSize() != newHeaderSize {
        t.Errorf("Header size should be %d after SetupOcto, got %d", newHeaderSize, GetMaxHeaderSize())
    }
    
    // Reset to defaults
    SetupOcto(logger, 10*1024*1024, 1024*1024)
}