/**
* @Author: Vincent
* @Date: 2023/9/14 16:30
 */
package reflect

type CassandraEncDec interface {
	MarshalCas() ([]byte, error)
	UnmarshalCas([]byte) error
}
