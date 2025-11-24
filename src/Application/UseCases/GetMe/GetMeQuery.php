<?php

declare(strict_types=1);
declare(ticks=1000);

namespace App\Application\UseCases\GetMe;

readonly class GetMeQuery
{
    /**
     * @param int $id
     * @param string $email
     * @param string $phone
     * @param string $firstName
     * @param string $lastName
     * @param string[] $roles
     */
    public function __construct(
        public int $id,
        public string $email,
        public string $phone,
        public string $firstName,
        public string $lastName,
        public array $roles
    ){}
}
