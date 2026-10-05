// Package contract checks the interfaces of the energystore against copies of its neighbours'
// code (M6 of the test-environment plan): the calls of eegfaktura-web and eegfaktura-v3, the
// fields v3 reads from the responses, the CR_MSG payload of eda-xp and v3's energy-mock, the
// cr_msg_history reply v3 reads and the backend's master-data proto. Test files only; fixtures and
// their origin in testdata/README.md.
package contract
