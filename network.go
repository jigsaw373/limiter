package limiter

import (
	"fmt"
	"github.com/golang-jwt/jwt"
	"github.com/pkg/errors"
	"net"
	"net/http"
	"strings"
)

var (
	// DefaultIPv4Mask defines the default IPv4 mask used to obtain user IP.
	DefaultIPv4Mask = net.CIDRMask(32, 32)
	// DefaultIPv6Mask defines the default IPv6 mask used to obtain user IP.
	DefaultIPv6Mask = net.CIDRMask(128, 128)
	// ErrInvalidJWT defines an error returned when JWT is invalid.
	ErrInvalidJWT = fmt.Errorf("invalid JWT token")
)

// GetIP returns IP address from request.
// If options is defined and either TrustForwardHeader is true or ClientIPHeader is defined,
// it will lookup IP in HTTP headers.
// Please be advised that using this option could be insecure (ie: spoofed) if your reverse
// proxy is not configured properly to forward a trustworthy client IP.
// Please read the section "Limiter behind a reverse proxy" in the README for further information.
func (limiter *Limiter) GetIP(r *http.Request) net.IP {
	return GetIP(r, limiter.Options)
}

// GetJWTSub returns sub from request JWT.
// it will lookup sub in jwt token.
func (limiter *Limiter) GetJWTSub(r *http.Request) string {
	sub, err := GetJWTSub(r, limiter.Options.JWTSecret)
	limiter.ErrValidation = err
	return sub
}

// GetIPWithMask returns IP address from request by applying a mask.
// If options is defined and either TrustForwardHeader is true or ClientIPHeader is defined,
// it will lookup IP in HTTP headers.
// Please be advised that using this option could be insecure (ie: spoofed) if your reverse
// proxy is not configured properly to forward a trustworthy client IP.
// Please read the section "Limiter behind a reverse proxy" in the README for further information.
func (limiter *Limiter) GetIPWithMask(r *http.Request) net.IP {
	return GetIPWithMask(r, limiter.Options)
}

// GetIPKey extracts IP from request and returns hashed IP to use as store key.
// If options is defined and either TrustForwardHeader is true or ClientIPHeader is defined,
// it will lookup IP in HTTP headers.
// Please be advised that using this option could be insecure (ie: spoofed) if your reverse
// proxy is not configured properly to forward a trustworthy client IP.
// Please read the section "Limiter behind a reverse proxy" in the README for further information.
func (limiter *Limiter) GetIPKey(r *http.Request) string {
	return limiter.GetIPWithMask(r).String()
}

// GetIP returns IP address from request.
// If options is defined and either TrustForwardHeader is true or ClientIPHeader is defined,
// it will lookup IP in HTTP headers.
// Please be advised that using this option could be insecure (ie: spoofed) if your reverse
// proxy is not configured properly to forward a trustworthy client IP.
// Please read the section "Limiter behind a reverse proxy" in the README for further information.
func GetIP(r *http.Request, options ...Options) net.IP {
	if len(options) >= 1 {
		if options[0].ClientIPHeader != "" {
			ip := getIPFromHeader(r, options[0].ClientIPHeader)
			if ip != nil {
				return ip
			}
		}
		if options[0].TrustForwardHeader {
			ip := getIPFromXFFHeader(r)
			if ip != nil {
				return ip
			}

			ip = getIPFromHeader(r, "X-Real-IP")
			if ip != nil {
				return ip
			}
		}
	}

	remoteAddr := strings.TrimSpace(r.RemoteAddr)
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return net.ParseIP(remoteAddr)
	}

	return net.ParseIP(host)
}

// GetJWTSub returns sub from request JWT.
func GetJWTSub(r *http.Request, secret string) (string, error) {
	if token, valid := getAuthorizationToken(r); valid {
		sub, err := extractSubFromJWT(token, secret)
		return sub, err
	}
	return "", ErrInvalidJWT
}

// GetIPWithMask returns IP address from request by applying a mask.
// If options is defined and either TrustForwardHeader is true or ClientIPHeader is defined,
// it will lookup IP in HTTP headers.
// Please be advised that using this option could be insecure (ie: spoofed) if your reverse
// proxy is not configured properly to forward a trustworthy client IP.
// Please read the section "Limiter behind a reverse proxy" in the README for further information.
func GetIPWithMask(r *http.Request, options ...Options) net.IP {
	if len(options) == 0 {
		return GetIP(r)
	}

	ip := GetIP(r, options[0])
	if ip.To4() != nil {
		return ip.Mask(options[0].IPv4Mask)
	}
	if ip.To16() != nil {
		return ip.Mask(options[0].IPv6Mask)
	}
	return ip
}

func getIPFromXFFHeader(r *http.Request) net.IP {
	headers := r.Header.Values("X-Forwarded-For")
	if len(headers) == 0 {
		return nil
	}

	parts := []string{}
	for _, header := range headers {
		parts = append(parts, strings.Split(header, ",")...)
	}

	for i := range parts {
		part := strings.TrimSpace(parts[i])
		ip := net.ParseIP(part)
		if ip != nil {
			return ip
		}
	}

	return nil
}

func getIPFromHeader(r *http.Request, name string) net.IP {
	header := strings.TrimSpace(r.Header.Get(name))
	if header == "" {
		return nil
	}

	ip := net.ParseIP(header)
	if ip != nil {
		return ip
	}

	return nil
}

func extractSubFromJWT(jwtString string, secret string) (string, error) {
	claims := &jwt.StandardClaims{}
	token, err := jwt.ParseWithClaims(jwtString, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil {
		return "", err
	}
	if !token.Valid {
		return "", ErrInvalidJWT
	}
	return fmt.Sprint([]byte(claims.Subject)), nil
}

func getAuthorizationToken(r *http.Request) (string, bool) {
	bearer := "bearer "
	headerToken := r.Header.Get("Authorization")
	if headerToken == "" {
		return "", false
	}

	// Verify the token format (Bearer <token>)
	lowerToken := strings.ToLower(headerToken[:len(bearer)])
	if len(headerToken) <= len(bearer) || lowerToken != bearer {
		return "", false
	}
	tokenString := headerToken[len(bearer):]

	return tokenString, true
}
