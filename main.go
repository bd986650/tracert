package main

import (
	"fmt"
	"net"
	"os"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

func traceroute(target string, maxHops int, timeout time.Duration) error {
	tgtAddr := net.ParseIP(target)
	if tgtAddr == nil {
		addrs, err := net.LookupHost(target)
		if err != nil {
			return fmt.Errorf("не удалось разрешить хост: %v", err)
		}
		tgtAddr = net.ParseIP(addrs[0])
	}

	if tgtAddr == nil {
		return fmt.Errorf("не удалось получить IP адрес для %s", target)
	}

	fmt.Printf("traceroute to %s (%s), %d hops max, 40 byte packets\n", target, tgtAddr, maxHops)

	// выполнение трассировки
	for ttl := 1; ttl <= maxHops; ttl++ {
		// отправка пакета
		err := tracerouteHop(tgtAddr, ttl, timeout)
		if err != nil {
			fmt.Printf("%d: ошибка: %v\n", ttl, err)
		}
	}
	return nil
}

func tracerouteHop(target net.IP, ttl int, timeout time.Duration) error {
	conn, err := net.ListenPacket("ip4:icmp", "")
	if err != nil {
		return fmt.Errorf("не удалось открыть пакетное соединение: %v", err)
	}
	defer conn.Close()

	// новый IPv4 пакетный соединитель
	pConn := ipv4.NewPacketConn(conn)
	pConn.SetTTL(ttl)

	// новый ICMP Echo запрос
	id := os.Getpid() & 0xffff
	seq := ttl

	msg := &icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{
			ID:   id,
			Seq:  seq,
			Data: []byte("traceroute"),
		},
	}

	msgBytes, _ := msg.Marshal(nil)

	addr := &net.IPAddr{IP: target}

	// отправка пакета
	_, err = pConn.WriteTo(msgBytes, nil, addr)
	if err != nil {
		return fmt.Errorf("не удалось отправить ICMP запрос: %v", err)
	}

	buffer := make([]byte, 1500)
	conn.SetDeadline(time.Now().Add(timeout))

	// ответ от маршрутизатора
	n, rAddr, err := conn.ReadFrom(buffer)
	if err != nil {
		return fmt.Errorf("не получен ответ: %v", err)
	}

	ipAddr, ok := rAddr.(*net.IPAddr)
	if !ok {
		return fmt.Errorf("не удалось привести адрес к типу *net.IPAddr")
	}

	// парс ICMP сообщение
	msgResponse, _ := icmp.ParseMessage(1, buffer[:n])
	fmt.Printf("Получен ICMP тип: %d\n", msgResponse.Type)

	switch body := msgResponse.Body.(type) {
	case *icmp.Echo:
		if body.ID == id {
			// адрес и время
			fmt.Printf("%d  %s (ID: %d) %.3f ms\n", ttl, ipAddr, body.ID, float64(time.Since(time.Now()).Milliseconds()))
		}
	case *icmp.TimeExceeded:
		// если тип "TimeExceeded", это ok
		fmt.Printf("%d  время жизни пакета истекло от %s\n", ttl, ipAddr)
	default:
		// для других типов сообщений
		return fmt.Errorf("неизвестный тип ICMP ответа: %v", body)
	}

	return nil
}

func main() {
	target := "google.com"
	maxHops := 32
	timeout := 2 * time.Second

	err := traceroute(target, maxHops, timeout)
	if err != nil {
		fmt.Println("Ошибка при выполнении трассировки:", err)
		os.Exit(1)
	}
}
