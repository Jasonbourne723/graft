package raft

import (
	"context"
	"fmt"
	"sync"
	"time"

	"math/rand"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Role int8

const (
	Leader Role = iota
	Follower
	Candidate
)

const (
	dateFormat = "2006-01-02 15:04:05"
)

var _ RaftServer = NewServer()

func NewServer(options ...func(*Server)) *Server {
	server := &Server{
		term:      0,
		termLock:  &sync.RWMutex{},
		role:      Follower,
		heartChan: make(chan int32, 1),
	}

	for _, option := range options {
		option(server)
	}
	return server
}

func WithOthers(others []*Node) func(*Server) {

	for _, node := range others {
		conn, err := grpc.NewClient(node.EndPoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			fmt.Println("Failed to connect to ", node.EndPoint)
		}
		node.Client = NewRaftClient(conn)
	}

	return func(s *Server) {
		s.others = others
	}
}

func WithId(id int32) func(*Server) {
	return func(s *Server) {
		s.Id = id
	}
}

type Server struct {
	Id        int32
	term      int32
	termLock  *sync.RWMutex
	role      Role
	others    []*Node
	heartChan chan int32
	UnimplementedRaftServer
}

type Node struct {
	EndPoint string
	Id       int
	Client   RaftClient
}

// Start starts the server
func (s *Server) Start() {
	// Code
	go s.checkHeart()
}

// checkHeart checks for heartbeats from leader
func (s *Server) checkHeart() {
	// Code
	for {
		timeOut := time.Duration(rand.Intn(3000)+3000) * time.Millisecond
		select {
		case <-time.After(timeOut):
			if s.role != Leader {
				fmt.Println(time.Now().Format(dateFormat) + "Start Election because of timeout")
				s.startElection()
			}
		case term := <-s.heartChan:
			fmt.Println(time.Now().Format(dateFormat) + "Receive heart from leader")
			if s.role != Leader {
				s.compareAndSetTerm(term)
			}
		}
	}
}

// sendHeart sends heartbeats to all other nodes
func (s *Server) sendHeart() {
	// Code
	for {
		for _, node := range s.others {
			go node.Client.RequestHeart(context.Background(), &RequestHeartRequest{Term: s.getTerm()})
		}
		<-time.After(2000 * time.Millisecond)
		if s.role != Leader {
			break
		}
	}
}

// addAndGetTerm adds and gets the term
func (s *Server) addAndGetTerm() int32 {
	s.termLock.Lock()
	defer s.termLock.Unlock()
	s.term++
	return s.term
}

// getTerm gets the term
func (s *Server) getTerm() int32 {
	s.termLock.RLock()
	defer s.termLock.RUnlock()
	return s.term
}

// compareAndSetTerm compares and sets the term
func (s *Server) compareAndSetTerm(term int32) bool {
	s.termLock.Lock()
	defer s.termLock.Unlock()
	if term > s.term {
		s.term = term
		return true
	}
	return false
}

// startElection starts the election process
func (s *Server) startElection() {
	// Increment term
	term := s.addAndGetTerm()
	fmt.Println("Start Election Id: ", s.Id, ",Term: ", term)
	s.role = Candidate
	voteCount := 1
	voteChan := make(chan bool, len(s.others))
	go s.sendRequestVote(voteChan, term)

	for voteResult := range voteChan {
		if voteResult {
			voteCount++
			if voteCount > len(s.others)/2 {
				s.becomeLeader()
				break
			}
		}
	}
	fmt.Println("Election finished")
}

// becomeLeader becomes the leader
func (s *Server) becomeLeader() {
	s.role = Leader
	fmt.Println("Leader is ", s.Id)
	go s.sendHeart()
}

// sendRequestVote sends request vote to all other nodes
func (s *Server) sendRequestVote(voteChan chan<- bool, term int32) {
	wg := &sync.WaitGroup{}
	for _, node := range s.others {
		fmt.Println("Send RequestVote to ", node.EndPoint)
		wg.Add(1)
		go func(node *Node) {
			defer wg.Done()
			defer func(voteChan chan<- bool) {
				if err := recover(); err != nil {
					fmt.Println("Failed to send RequestVote to ", node.EndPoint)
					voteChan <- false
				}
			}(voteChan)
			context, _ := context.WithTimeout(context.Background(), 2*time.Second)
			respose, err := node.Client.RequestVote(context, &RequestVoteRequest{Term: term, CandidateId: s.Id})
			if err != nil {
				voteChan <- false
			} else {
				if respose.VoteGranted {
					voteChan <- true
				} else {
					voteChan <- false
				}
			}
		}(node)
	}
	wg.Wait()
	close(voteChan)
}

// VoteFor votes for a candidate
func (s *Server) VoteFor(term int32, candidateId int32) bool {
	// If the candidate's term is more than the server's term, then vote
	if s.compareAndSetTerm(term) {
		s.role = Follower
		fmt.Println("Vote for ", candidateId, " in term ", term)
		return true
	}
	fmt.Println("Don't vote for ", candidateId, " in term ", term)
	return false
}

// RequestVote handles the request vote RPC
func (s *Server) RequestVote(context context.Context, request *RequestVoteRequest) (*RequestVoteResponse, error) {
	result := s.VoteFor(request.Term, request.CandidateId)
	return &RequestVoteResponse{VoteGranted: result}, nil
}

// RequestHeart handles the request heart RPC
func (s *Server) RequestHeart(context context.Context, request *RequestHeartRequest) (*RequestHeartResponse, error) {
	s.heartChan <- request.Term
	return &RequestHeartResponse{}, nil
}
