<?php

declare(strict_types=1);
declare(ticks=1000);

namespace App\Domain\Repository;

use App\Domain\Entity\User;

interface UserRepositoryInterface
{
    public function store(User $user, string $hashPassword): int;
}
