# Lottery Ticket Search Design

## 1. Overview

เอกสารนี้อธิบายการออกแบบระบบค้นหาและจัดสรร Lottery Ticket จากข้อมูลมากกว่า 10 ล้านรายการ

### Requirement

1. Lottery Ticket แต่ละใบเป็นเลข 6 หลัก
2. รองรับข้อมูลจำนวน 10 ล้านรายการ
3. รองรับการค้นหาด้วย pattern ความยาว 6 ตัวอักษร โดยสามารถใช้ wildcard (`*`) ได้

```
****23
1****5
123***
```

1. ระบบต้องป้องกันไม่ให้ Lottery Ticket ใบเดียวกันถูกจัดสรรให้ผู้ใช้หลายคนพร้อมกัน

---

## 2. Proposed Architecture

```
Client
  |
  v
Lottery Search API
  |
  v
PostgreSQL
```

เลือกใช้ PostgreSQL เป็นฐานข้อมูลหลัก เนื่องจากรองรับ:

- Transactional consistency
- การทำ Index เพื่อเพิ่มประสิทธิภาพในการค้นหา
- `FOR UPDATE SKIP LOCKED` สำหรับจัดการ concurrent allocation

สำหรับข้อมูลประมาณ 10 ล้านรายการ PostgreSQL สามารถรองรับได้ หากออกแบบ index ให้เหมาะสมกับรูปแบบการค้นหา

---



## 3. Data Model

ตัวอย่าง Table:

```
CREATE TABLE lottery_tickets (
    id BIGSERIAL PRIMARY KEY,
    number CHAR(6) NOT NULL,

    digit1 SMALLINT NOT NULL,
    digit2 SMALLINT NOT NULL,
    digit3 SMALLINT NOT NULL,
    digit4 SMALLINT NOT NULL,
    digit5 SMALLINT NOT NULL,
    digit6 SMALLINT NOT NULL,

    status VARCHAR(20) NOT NULL DEFAULT 'AVAILABLE',

    reserved_by VARCHAR(100),
    reserved_until TIMESTAMP
);
```

แยกเลขแต่ละหลักออกเป็น `digit1` ถึง `digit6` เพื่อรองรับการค้นหา wildcard ในแต่ละตำแหน่ง และสามารถสร้าง index ตามตำแหน่งที่ใช้ค้นหาได้

ตัวอย่าง:

```
Lottery number: 123456

digit1 = 1
digit2 = 2
digit3 = 3
digit4 = 4
digit5 = 5
digit6 = 6
```

---



## 4. Pattern Matching

Search pattern จะมีความยาว 6 ตัวอักษรเสมอ

แต่ละตำแหน่งสามารถเป็น:

- `0-9`
- `*`

โดย `*` หมายถึงเลขอะไรก็ได้ในตำแหน่งนั้น

ตัวอย่าง:

```
1****5
```

จะถูกแปลงเป็น:

```
WHERE digit1 = 1
  AND digit6 = 5
```

การค้นหาและ reserve จะทำภายใน transaction เดียวกัน

```
BEGIN;

SELECT id, number
FROM lottery_tickets
WHERE digit1 = 1
  AND digit6 = 5
  AND (status = 'AVAILABLE' OR (status = 'RESERVED' AND reserved_until < NOW()))
LIMIT 1
FOR UPDATE SKIP LOCKED;

UPDATE lottery_tickets
SET status = 'RESERVED',
    reserved_by = :user_id,
    reserved_until = NOW() + INTERVAL '5 minutes'
WHERE id = :ticket_id;

COMMIT;
```

`FOR UPDATE` ใช้สำหรับ lock row ที่ถูกเลือก เพื่อป้องกัน transaction อื่นนำ Lottery Ticket ใบเดียวกันไป allocate

`SKIP LOCKED` ทำให้ request อื่นสามารถข้าม row ที่ถูก lock อยู่ และค้นหา Lottery Ticket ใบถัดไปได้ทันที

ก่อน `COMMIT` ระบบจะเปลี่ยนสถานะ Lottery Ticket จาก `AVAILABLE` เป็น `RESERVED`

วิธีนี้ช่วยป้องกันไม่ให้ Lottery Ticket ใบเดียวกันถูกจัดสรรให้ผู้ใช้หลายคนพร้อมกัน

---



## 5. Indexing Strategy

เพื่อรองรับ wildcard ที่สามารถอยู่ในตำแหน่งใดก็ได้ จะสร้าง basic index สำหรับแต่ละ digit

```
CREATE INDEX idx_digit1 ON lottery_tickets (digit1);
CREATE INDEX idx_digit2 ON lottery_tickets (digit2);
CREATE INDEX idx_digit3 ON lottery_tickets (digit3);
CREATE INDEX idx_digit4 ON lottery_tickets (digit4);
CREATE INDEX idx_digit5 ON lottery_tickets (digit5);
CREATE INDEX idx_digit6 ON lottery_tickets (digit6);
```

PostgreSQL สามารถเลือกใช้ index ตามตำแหน่งที่ถูกระบุใน search pattern

ตัวอย่าง:

```
1****5
```

จะกลายเป็น:

```
WHERE digit1 = 1
  AND digit6 = 5
  AND status = 'AVAILABLE'
```

PostgreSQL สามารถใช้ index ของ `digit1` และ `digit6` เพื่อช่วยลดจำนวนข้อมูลที่ต้องค้นหา

สำหรับ pattern ที่ถูกค้นหาบ่อย สามารถเพิ่ม composite index เพื่อเพิ่มประสิทธิภาพได้

ตัวอย่าง:

```
CREATE INDEX idx_last_two_digits
ON lottery_tickets (digit5, digit6);
```

เหมาะกับ:

```
****23
```

อีกตัวอย่าง:

```
CREATE INDEX idx_first_three_digits
ON lottery_tickets (digit1, digit2, digit3);
```

เหมาะกับ:

```
123***
```

ไม่ควรสร้าง composite index สำหรับ wildcard ทุก combination เพราะจะทำให้มี index จำนวนมากเกินไป

ควรสร้างเพิ่มเฉพาะ pattern ที่ถูกค้นหาบ่อยจากการใช้งานจริง

### Indexing Trade-off

การมี index จำนวนมากช่วยให้ search เร็วขึ้น แต่ทำให้ insert และ update ช้าลง

ดังนั้นควรใช้ single-digit index เป็นพื้นฐาน และเพิ่ม composite index เฉพาะ search pattern ที่มีการใช้งานบ่อย